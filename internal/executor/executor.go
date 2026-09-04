package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"

	libarazzo "github.com/pb33f/libopenapi/arazzo"
)

// Executor runs Arazzo HTTP steps with a cookie jar and operationId resolution.
type Executor struct {
	mu            sync.Mutex
	client        *http.Client
	baseURL       string
	sources       []*libarazzo.ResolvedSource
	logger        *slog.Logger
	arazzoParamIn map[string]string
}

// New returns an HTTP executor. A cookie jar is attached if the client lacks one.
// arazzoParamIn maps parameter names to Arazzo `in` locations (header, query, path, cookie).
func New(client *http.Client, baseURL string, sources []*libarazzo.ResolvedSource, logger *slog.Logger, arazzoParamIn map[string]string) *Executor {
	return &Executor{
		client:        ensureCookieJar(client),
		baseURL:       baseURL,
		sources:       sources,
		logger:        logger,
		arazzoParamIn: arazzoParamIn,
	}
}

func ensureCookieJar(client *http.Client) *http.Client {
	if client == nil {
		jar, _ := cookiejar.New(nil)
		return &http.Client{Jar: jar}
	}
	if client.Jar != nil {
		return client
	}
	clone := *client
	jar, _ := cookiejar.New(nil)
	clone.Jar = jar
	return &clone
}

// SetBaseURL updates the API root used to join relative operation paths.
func (e *Executor) SetBaseURL(baseURL string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.baseURL = baseURL
}

// BaseURL returns the current API root.
func (e *Executor) BaseURL() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.baseURL
}

// Client returns the HTTP client used for workflow steps (including its cookie jar).
func (e *Executor) Client() *http.Client {
	return e.client
}

func (e *Executor) Execute(ctx context.Context, req *libarazzo.ExecutionRequest) (*libarazzo.ExecutionResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("nil execution request")
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	path := req.OperationPath
	var paramIn map[string]string

	if req.OperationID != "" && (method == "" || path == "") {
		op, err := resolveOperation(e.sources, req.Source, req.OperationID)
		if err != nil {
			return nil, err
		}
		if method == "" {
			method = op.method
		}
		if path == "" {
			path = op.path
		}
		paramIn = op.paramIn
	}
	if method == "" || path == "" {
		return nil, fmt.Errorf("could not resolve HTTP method and path for operation %q", req.OperationID)
	}

	pathParams := make(map[string]string)
	query := url.Values{}
	headers := make(http.Header)
	var cookies []*http.Cookie

	for name, value := range req.Parameters {
		loc := paramLocation(name, path, paramIn, e.arazzoParamIn)
		rendered := fmt.Sprintf("%v", value)
		switch loc {
		case "path":
			pathParams[name] = rendered
		case "header":
			headers.Add(name, rendered)
		case "cookie":
			cookies = append(cookies, &http.Cookie{Name: name, Value: rendered})
		default:
			query.Add(name, rendered)
		}
	}

	for name, value := range pathParams {
		path = strings.ReplaceAll(path, "{"+name+"}", url.PathEscape(value))
	}

	e.mu.Lock()
	baseURL := e.baseURL
	e.mu.Unlock()

	target, err := url.Parse(joinBaseAndPath(baseURL, path))
	if err != nil {
		return nil, err
	}
	if len(query) > 0 {
		existing := target.Query()
		for k, vs := range query {
			for _, v := range vs {
				existing.Add(k, v)
			}
		}
		target.RawQuery = existing.Encode()
	}

	var bodyReader io.Reader
	if req.RequestBody != nil {
		switch b := req.RequestBody.(type) {
		case []byte:
			bodyReader = bytes.NewReader(b)
		case string:
			bodyReader = strings.NewReader(b)
		default:
			jsonBytes, err := json.Marshal(req.RequestBody)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
			bodyReader = bytes.NewReader(jsonBytes)
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, target.String(), bodyReader)
	if err != nil {
		return nil, err
	}
	if req.ContentType != "" {
		httpReq.Header.Set("Content-Type", req.ContentType)
	} else if req.RequestBody != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	for k, vs := range headers {
		for _, v := range vs {
			httpReq.Header.Add(k, v)
		}
	}
	for _, c := range cookies {
		httpReq.AddCookie(c)
	}

	if e.logger != nil {
		e.logger.Info("arazzo request",
			"method", method,
			"url", redactURL(target.String()),
			"operationId", req.OperationID,
			"body", redactJSON(req.RequestBody),
		)
	}

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if e.logger != nil {
		e.logger.Info("arazzo response",
			"status", resp.StatusCode,
			"bytes", len(respBytes),
			"operationId", req.OperationID,
		)
	}

	var body any
	if len(respBytes) > 0 {
		ct := resp.Header.Get("Content-Type")
		if strings.Contains(ct, "json") || json.Valid(respBytes) {
			if err := json.Unmarshal(respBytes, &body); err != nil {
				body = respBytes
			}
		} else {
			body = respBytes
		}
	}

	headerCopy := make(map[string][]string, len(resp.Header))
	for k, v := range resp.Header {
		headerCopy[k] = append([]string(nil), v...)
	}

	return &libarazzo.ExecutionResponse{
		StatusCode: resp.StatusCode,
		Headers:    headerCopy,
		Body:       body,
		URL:        target.String(),
		Method:     method,
	}, nil
}

func joinBaseAndPath(baseURL, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if baseURL == "" {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(baseURL, "/") + path
}
