package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pb33f/libopenapi"
	libarazzo "github.com/pb33f/libopenapi/arazzo"
	higharazzo "github.com/pb33f/libopenapi/datamodel/high/arazzo"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/shaunhoulihan/arazzo-go/internal/executor"
	"github.com/shaunhoulihan/arazzo-go/internal/sources"
)

// Result is the outcome of a single workflow run.
type Result = libarazzo.WorkflowResult

// WorkflowInfo is metadata about a workflow in an Arazzo document.
type WorkflowInfo struct {
	WorkflowID  string
	Summary     string
	Description string
	StepIDs     []string
	Steps       []StepInfo
	OutputNames []string
}

// StepInfo is a brief description of one workflow step.
type StepInfo struct {
	StepID        string
	Description   string
	OperationID   string
	OperationPath string
	WorkflowID    string
}

// Runner loads an Arazzo document and executes its workflows.
// An Engine (and therefore a Runner) is not safe for concurrent use.
type Runner struct {
	path     string
	doc      *higharazzo.Arazzo
	sources  []*libarazzo.ResolvedSource
	engine   *libarazzo.Engine
	executor *executor.Executor
}

// FromFile parses, optionally validates, and prepares an Arazzo document from a file path.
func FromFile(path string, opts ...Option) (*Runner, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read arazzo file: %w", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return fromBytes(abs, data, opts...)
}

// FromBytes parses, optionally validates, and prepares an Arazzo document from memory.
// name is used as the document path for relative source resolution.
func FromBytes(name string, data []byte, opts ...Option) (*Runner, error) {
	if name == "" {
		name = "arazzo.yaml"
	}
	return fromBytes(name, data, opts...)
}

func fromBytes(path string, data []byte, opts ...Option) (*Runner, error) {
	cfg := defaultOptions()
	cfg.apply(opts)

	doc, err := libopenapi.NewArazzoDocument(data)
	if err != nil {
		return nil, fmt.Errorf("parse arazzo document: %w", err)
	}

	if !cfg.skipValidate {
		if result := libarazzo.Validate(doc); result != nil && result.HasErrors() {
			var hard []string
			for _, e := range result.Errors {
				if e == nil {
					continue
				}
				msg := e.Error()
				if isLenientValidationError(msg) {
					continue
				}
				hard = append(hard, fmt.Sprintf("line %d: %s", e.Line, e.Cause))
			}
			if len(hard) > 0 {
				return nil, fmt.Errorf("invalid arazzo document:\n  %s", strings.Join(hard, "\n  "))
			}
		}
	}

	client := cfg.httpClient
	if client == nil {
		jar, _ := cookiejar.New(nil)
		client = &http.Client{
			Timeout: cfg.timeout,
			Jar:     jar,
		}
	} else if client.Timeout == 0 && cfg.timeout > 0 {
		clone := *client
		clone.Timeout = cfg.timeout
		client = &clone
	}

	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		if cwd, err := os.Getwd(); err == nil {
			dir = cwd
		}
	}
	fsRoots := []string{dir}
	if cwd, err := os.Getwd(); err == nil && cwd != dir {
		fsRoots = append(fsRoots, cwd)
	}

	resolveCfg := &libarazzo.ResolveConfig{
		HTTPClient:  client,
		HTTPHandler: sources.NewHTTPHandler(client),
		FSRoots:     fsRoots,
		Timeout:     30 * time.Second,
		OpenAPIFactory: func(sourceURL string, b []byte) (*v3.Document, error) {
			document, err := libopenapi.NewDocument(b)
			if err != nil {
				return nil, fmt.Errorf("parse openapi %s: %w", sourceURL, err)
			}
			model, err := document.BuildV3Model()
			if err != nil {
				return nil, fmt.Errorf("build openapi model %s: %w", sourceURL, err)
			}
			return &model.Model, nil
		},
		ArazzoFactory: func(sourceURL string, b []byte) (*higharazzo.Arazzo, error) {
			return libopenapi.NewArazzoDocument(b)
		},
	}

	resolved, err := libarazzo.ResolveSources(doc, resolveCfg)
	if err != nil {
		return nil, fmt.Errorf("resolve source descriptions: %w", err)
	}

	exec := executor.New(client, cfg.baseURL, resolved, cfg.logger)
	engineCfg := &libarazzo.EngineConfig{}
	if cfg.sleepFunc != nil {
		engineCfg.SleepFunc = cfg.sleepFunc
	}
	engine := libarazzo.NewEngineWithConfig(doc, exec, resolved, engineCfg)

	return &Runner{
		path:     path,
		doc:      doc,
		sources:  resolved,
		engine:   engine,
		executor: exec,
	}, nil
}

func isLenientValidationError(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "operationid") && strings.Contains(lower, "$sourcedescriptions")
}

// ExecuteWorkflow runs a workflow by ID.
// If inputs contains "baseUrl", that value is used as the HTTP API root for this run.
func (r *Runner) ExecuteWorkflow(ctx context.Context, workflowID string, inputs map[string]any) (*Result, error) {
	if r == nil || r.engine == nil {
		return nil, fmt.Errorf("nil runner")
	}
	if workflowID == "" {
		return nil, fmt.Errorf("workflow id is required")
	}
	if inputs == nil {
		inputs = map[string]any{}
	}
	if v, ok := inputs["baseUrl"].(string); ok && strings.TrimSpace(v) != "" {
		r.executor.SetBaseURL(strings.TrimSpace(v))
	}
	if r.executor.BaseURL() == "" {
		return nil, fmt.Errorf("API base URL is required: set inputs[\"baseUrl\"] or use WithBaseURL")
	}
	return r.engine.RunWorkflow(ctx, workflowID, inputs)
}

// ListWorkflows returns metadata for every workflow in the document.
func (r *Runner) ListWorkflows() []WorkflowInfo {
	if r == nil || r.doc == nil {
		return nil
	}
	out := make([]WorkflowInfo, 0, len(r.doc.Workflows))
	for _, wf := range r.doc.Workflows {
		if wf == nil {
			continue
		}
		out = append(out, workflowInfo(wf))
	}
	return out
}

// DescribeWorkflow returns metadata for a single workflow.
func (r *Runner) DescribeWorkflow(workflowID string) (*WorkflowInfo, error) {
	if r == nil || r.doc == nil {
		return nil, fmt.Errorf("nil runner")
	}
	for _, wf := range r.doc.Workflows {
		if wf != nil && wf.WorkflowId == workflowID {
			info := workflowInfo(wf)
			return &info, nil
		}
	}
	return nil, fmt.Errorf("workflow %q not found", workflowID)
}

// HTTPClient returns the client used for workflow steps (including its cookie jar).
func (r *Runner) HTTPClient() *http.Client {
	if r == nil || r.executor == nil {
		return nil
	}
	return r.executor.Client()
}

// CookieJar returns the cookie jar used across workflow steps, if any.
func (r *Runner) CookieJar() http.CookieJar {
	client := r.HTTPClient()
	if client == nil {
		return nil
	}
	return client.Jar
}

// Document returns the parsed Arazzo document.
func (r *Runner) Document() *higharazzo.Arazzo {
	if r == nil {
		return nil
	}
	return r.doc
}

func workflowInfo(wf *higharazzo.Workflow) WorkflowInfo {
	info := WorkflowInfo{
		WorkflowID:  wf.WorkflowId,
		Summary:     wf.Summary,
		Description: wf.Description,
	}
	for _, step := range wf.Steps {
		if step == nil {
			continue
		}
		info.StepIDs = append(info.StepIDs, step.StepId)
		info.Steps = append(info.Steps, StepInfo{
			StepID:        step.StepId,
			Description:   step.Description,
			OperationID:   step.OperationId,
			OperationPath: step.OperationPath,
			WorkflowID:    step.WorkflowId,
		})
	}
	if wf.Outputs != nil {
		for name := range wf.Outputs.FromOldest() {
			info.OutputNames = append(info.OutputNames, name)
		}
	}
	return info
}
