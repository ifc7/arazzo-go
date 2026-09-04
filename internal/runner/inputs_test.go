package runner

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/pb33f/libopenapi"
	higharazzo "github.com/pb33f/libopenapi/datamodel/high/arazzo"
)

func TestApplyWorkflowInputDefaultsInlineSchema(t *testing.T) {
	t.Parallel()
	doc := mustParseArazzoDoc(t, []byte(`arazzo: 1.0.1
info:
  title: defaults
  version: 1.0.0
sourceDescriptions:
  - name: api
    type: openapi
    url: spec.yaml
workflows:
  - workflowId: paint
    inputs:
      type: object
      properties:
        color:
          type: string
          default: red
        size:
          type: string
          default: m
    steps:
      - stepId: noop
        operationId: api.noop
`))
	got, err := applyWorkflowInputDefaults(doc, "paint", map[string]any{"size": "xl"})
	if err != nil {
		t.Fatal(err)
	}
	if got["color"] != "red" {
		t.Fatalf("color default = %#v", got["color"])
	}
	if got["size"] != "xl" {
		t.Fatalf("explicit size overwritten: %#v", got["size"])
	}
}

func TestApplyWorkflowInputDefaultsComponentRef(t *testing.T) {
	t.Parallel()
	doc := mustParseArazzoDoc(t, []byte(`arazzo: 1.0.1
info:
  title: defaults
  version: 1.0.0
sourceDescriptions:
  - name: api
    type: openapi
    url: spec.yaml
workflows:
  - workflowId: login
    inputs:
      $ref: '#/components/inputs/loginInputs'
    steps:
      - stepId: noop
        operationId: api.noop
components:
  inputs:
    loginInputs:
      type: object
      properties:
        baseUrl:
          type: string
          default: http://localhost:8080
`))
	got, err := applyWorkflowInputDefaults(doc, "login", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if got["baseUrl"] != "http://localhost:8080" {
		t.Fatalf("baseUrl default = %#v", got["baseUrl"])
	}
}

func TestExecuteWorkflowAppliesInputDefaults(t *testing.T) {
	t.Parallel()
	var gotColor string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/paint" {
			http.NotFound(w, r)
			return
		}
		body := decodeJSON(t, r)
		if c, _ := body["color"].(string); c != "" {
			gotColor = c
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	spec := []byte(`openapi: 3.0.3
info:
  title: x
  version: "1"
paths:
  /paint:
    post:
      operationId: paint
      responses:
        "200":
          description: ok
`)
	if err := os.WriteFile(filepath.Join(dir, "spec.yaml"), spec, 0o644); err != nil {
		t.Fatal(err)
	}
	arazzoDoc := []byte(`arazzo: 1.0.1
info:
  title: defaults
  version: 1.0.0
sourceDescriptions:
  - name: api
    type: openapi
    url: spec.yaml
workflows:
  - workflowId: paint
    inputs:
      type: object
      properties:
        color:
          type: string
          default: red
    steps:
      - stepId: paint
        operationId: api.paint
        requestBody:
          contentType: application/json
          payload:
            color: $inputs.color
        successCriteria:
          - condition: $statusCode == 200
`)
	path := filepath.Join(dir, "workflow.yaml")
	if err := os.WriteFile(path, arazzoDoc, 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := FromFile(path, WithHTTPClient(srv.Client()), WithBaseURL(srv.URL), WithLogger(slog.New(slog.DiscardHandler)))
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.ExecuteWorkflow(context.Background(), "paint", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("workflow failed: %v", result.Error)
	}
	if gotColor != "red" {
		t.Fatalf("request color = %q, want red from schema default", gotColor)
	}
}

func mustParseArazzoDoc(t *testing.T, data []byte) *higharazzo.Arazzo {
	t.Helper()
	doc, err := libopenapi.NewArazzoDocument(data)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}
