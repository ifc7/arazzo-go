package sources

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxSourceBody = 10 * 1024 * 1024

func looksLikeHTML(body []byte, contentType string) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}
	s := strings.TrimSpace(string(body))
	if len(s) < 5 {
		return false
	}
	ls := strings.ToLower(s)
	return strings.HasPrefix(ls, "<!doctype html") || strings.HasPrefix(ls, "<html")
}

func looksLikeOpenAPI(body []byte) bool {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return false
	}
	if s[0] == '{' || s[0] == '[' {
		ls := strings.ToLower(s)
		return strings.Contains(ls, `"openapi"`) || strings.Contains(ls, `"swagger"`)
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)
		return strings.HasPrefix(lower, "openapi:") || strings.HasPrefix(lower, "swagger:")
	}
	return false
}

// NewHTTPHandler fetches an OpenAPI source over HTTP.
// Accept is set so content-negotiating locators (ifc /i/... URLs) return the
// spec rather than HTML docs.
func NewHTTPHandler(client *http.Client) func(string) ([]byte, error) {
	if client == nil {
		client = http.DefaultClient
	}
	return func(sourceURL string) ([]byte, error) {
		body, ct, err := fetchURL(client, sourceURL)
		if err != nil {
			return nil, err
		}
		if looksLikeHTML(body, ct) {
			return nil, fmt.Errorf("source %q returned HTML, not an OpenAPI document", sourceURL)
		}
		return body, nil
	}
}

func fetchURL(client *http.Client, rawURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	// Do not advertise text/html. ifc locators serve HTML when Accept prefers
	// it (browser) and the raw spec otherwise (curl, this client).
	req.Header.Set("Accept", strings.Join([]string{
		"application/yaml",
		"application/x-yaml",
		"text/yaml",
		"application/vnd.oai.openapi",
		"application/vnd.oai.openapi+json",
		"application/json",
		"text/plain",
	}, ", "))

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxSourceBody+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", err
	}
	if int64(len(body)) > maxSourceBody {
		return nil, "", fmt.Errorf("response body exceeds max size of %d bytes", maxSourceBody)
	}
	return body, resp.Header.Get("Content-Type"), nil
}
