package runner

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func testWorkflowDoc(t *testing.T) *higharazzo.Arazzo {
	t.Helper()
	return mustParseArazzoDoc(t, []byte(`arazzo: 1.0.1
info:
  title: input tests
  version: 1.0.0
sourceDescriptions:
  - name: api
    type: openapi
    url: spec.yaml
workflows:
  - workflowId: test-wf
    inputs:
      type: object
      properties:
        baseUrl:
          type: string
        password:
          type: string
        portNumber:
          type: integer
        ratio:
          type: number
        isAdmin:
          type: boolean
        metadata:
          type: object
        tags:
          type: array
        theme:
          type: string
          default: dark
    steps:
      - stepId: s1
        operationId: api.noop
`))
}

func TestResolveWorkflowInputsEnv(t *testing.T) {
	t.Parallel()
	doc := testWorkflowDoc(t)

	env := []string{
		"ARAZZO_INPUT_BASEURL=https://api.example.com",
		"ARAZZO_INPUT_password=secret_password",
		"ARAZZO_INPUT_PORT_NUMBER=8080",
		"ARAZZO_INPUT_RATIO=3.14",
		"ARAZZO_INPUT_ISADMIN=true",
		`ARAZZO_INPUT_METADATA={"region":"us-east-1","tier":2}`,
		`ARAZZO_INPUT_TAGS=["go","arazzo"]`,
		"UNRELATED_VAR=ignored",
	}

	res, err := ResolveWorkflowInputs(doc, "test-wf", InputSources{Env: env})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res["baseUrl"] != "https://api.example.com" {
		t.Errorf("baseUrl: got %v, want https://api.example.com", res["baseUrl"])
	}
	if res["password"] != "secret_password" {
		t.Errorf("password: got %v, want secret_password", res["password"])
	}
	if res["portNumber"] != 8080 {
		t.Errorf("portNumber: got %v (%T), want 8080", res["portNumber"], res["portNumber"])
	}
	if res["ratio"] != 3.14 {
		t.Errorf("ratio: got %v, want 3.14", res["ratio"])
	}
	if res["isAdmin"] != true {
		t.Errorf("isAdmin: got %v, want true", res["isAdmin"])
	}
	expectedMeta := map[string]any{"region": "us-east-1", "tier": float64(2)}
	if !reflect.DeepEqual(res["metadata"], expectedMeta) {
		t.Errorf("metadata: got %#v, want %#v", res["metadata"], expectedMeta)
	}
	expectedTags := []any{"go", "arazzo"}
	if !reflect.DeepEqual(res["tags"], expectedTags) {
		t.Errorf("tags: got %#v, want %#v", res["tags"], expectedTags)
	}
	if res["theme"] != "dark" {
		t.Errorf("theme (default): got %v, want dark", res["theme"])
	}
	if _, exists := res["UNRELATED_VAR"]; exists {
		t.Errorf("unrelated var should not be in inputs: %v", res["UNRELATED_VAR"])
	}
}

func TestResolveWorkflowInputsCoercionErrors(t *testing.T) {
	t.Parallel()
	doc := testWorkflowDoc(t)

	// Invalid integer
	_, err := ResolveWorkflowInputs(doc, "test-wf", InputSources{
		Env: []string{"ARAZZO_INPUT_PORTNUMBER=not_an_int"},
	})
	if err == nil || !strings.Contains(err.Error(), "portNumber") || !strings.Contains(err.Error(), "integer") {
		t.Fatalf("expected integer coercion error, got: %v", err)
	}

	// Invalid boolean
	_, err = ResolveWorkflowInputs(doc, "test-wf", InputSources{
		Env: []string{"ARAZZO_INPUT_ISADMIN=maybe"},
	})
	if err == nil || !strings.Contains(err.Error(), "isAdmin") || !strings.Contains(err.Error(), "boolean") {
		t.Fatalf("expected boolean coercion error, got: %v", err)
	}

	// Invalid object
	_, err = ResolveWorkflowInputs(doc, "test-wf", InputSources{
		Env: []string{"ARAZZO_INPUT_METADATA=not_a_json_object"},
	})
	if err == nil || !strings.Contains(err.Error(), "metadata") || !strings.Contains(err.Error(), "object") {
		t.Fatalf("expected object coercion error, got: %v", err)
	}

	// Secret value must not be leaked in coercion error
	secretDoc := mustParseArazzoDoc(t, []byte(`arazzo: 1.0.1
info:
  title: secret test
  version: 1.0.0
sourceDescriptions:
  - name: api
    type: openapi
    url: spec.yaml
workflows:
  - workflowId: wf
    inputs:
      type: object
      properties:
        password:
          type: integer
    steps:
      - stepId: s1
        operationId: api.noop
`))
	_, err = ResolveWorkflowInputs(secretDoc, "wf", InputSources{
		Env: []string{"ARAZZO_INPUT_PASSWORD=super_secret_raw_credential"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "super_secret_raw_credential") {
		t.Fatalf("secret leaked in error message: %v", err)
	}
	if !strings.Contains(err.Error(), "***") {
		t.Fatalf("expected *** in error message, got: %v", err)
	}
}

func TestResolveWorkflowInputsEnvFile(t *testing.T) {
	t.Parallel()
	doc := testWorkflowDoc(t)

	envContent := `
# Lines with comments and empty lines are ignored
export ARAZZO_INPUT_BASEURL=https://envfile.example.com
password='single_quoted#password'
portNumber = 9000
isAdmin="true"
ratio=1.25 # trailing comment
metadata="{\"key\":\"value\"}"
tags='["t1", "t2"]'
IGNORED_VARIABLE=hello
`
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte(envContent), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := ResolveWorkflowInputs(doc, "test-wf", InputSources{
		Env:     []string{},
		EnvFile: envPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res["baseUrl"] != "https://envfile.example.com" {
		t.Errorf("baseUrl: got %v, want https://envfile.example.com", res["baseUrl"])
	}
	if res["password"] != "single_quoted#password" {
		t.Errorf("password: got %v, want single_quoted#password", res["password"])
	}
	if res["portNumber"] != 9000 {
		t.Errorf("portNumber: got %v (%T), want 9000", res["portNumber"], res["portNumber"])
	}
	if res["isAdmin"] != true {
		t.Errorf("isAdmin: got %v, want true", res["isAdmin"])
	}
	if res["ratio"] != 1.25 {
		t.Errorf("ratio: got %v, want 1.25", res["ratio"])
	}
	if !reflect.DeepEqual(res["metadata"], map[string]any{"key": "value"}) {
		t.Errorf("metadata: got %#v", res["metadata"])
	}
	if !reflect.DeepEqual(res["tags"], []any{"t1", "t2"}) {
		t.Errorf("tags: got %#v", res["tags"])
	}
	if _, exists := res["IGNORED_VARIABLE"]; exists {
		t.Errorf("unmatched variable in .env should not be included: %v", res["IGNORED_VARIABLE"])
	}
}

func TestResolveWorkflowInputsInputsFile(t *testing.T) {
	t.Parallel()
	doc := testWorkflowDoc(t)
	tmpDir := t.TempDir()

	// YAML inputs file
	yamlPath := filepath.Join(tmpDir, "inputs.yaml")
	if err := os.WriteFile(yamlPath, []byte("baseUrl: https://yaml.example.com\nportNumber: 3000\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := ResolveWorkflowInputs(doc, "test-wf", InputSources{
		InputsFile: yamlPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res["baseUrl"] != "https://yaml.example.com" {
		t.Errorf("baseUrl: got %v", res["baseUrl"])
	}
	if res["portNumber"] != 3000 {
		t.Errorf("portNumber: got %v", res["portNumber"])
	}

	// JSON inputs file
	jsonPath := filepath.Join(tmpDir, "inputs.json")
	if err := os.WriteFile(jsonPath, []byte(`{"baseUrl":"https://json.example.com","isAdmin":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = ResolveWorkflowInputs(doc, "test-wf", InputSources{
		InputsFile: jsonPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res["baseUrl"] != "https://json.example.com" {
		t.Errorf("baseUrl: got %v", res["baseUrl"])
	}
	if res["isAdmin"] != true {
		t.Errorf("isAdmin: got %v", res["isAdmin"])
	}

	// Stdin input with "-"
	stdinContent := `{"baseUrl":"https://stdin.example.com","password":"via-stdin"}`
	res, err = ResolveWorkflowInputs(doc, "test-wf", InputSources{
		InputsFile: "-",
		Stdin:      strings.NewReader(stdinContent),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res["baseUrl"] != "https://stdin.example.com" {
		t.Errorf("baseUrl: got %v", res["baseUrl"])
	}
	if res["password"] != "via-stdin" {
		t.Errorf("password: got %v", res["password"])
	}
}

func TestResolveWorkflowInputsPrecedence(t *testing.T) {
	t.Parallel()
	doc := mustParseArazzoDoc(t, []byte(`arazzo: 1.0.1
info:
  title: precedence
  version: 1.0.0
sourceDescriptions:
  - name: api
    type: openapi
    url: spec.yaml
workflows:
  - workflowId: wf
    inputs:
      type: object
      properties:
        a:
          type: string
        b:
          type: string
        c:
          type: string
        d:
          type: string
        e:
          type: string
          default: from-schema-default
    steps:
      - stepId: s1
        operationId: api.noop
`))

	tmpDir := t.TempDir()

	// Env provides a, b, c, d
	env := []string{
		"ARAZZO_INPUT_A=from-env",
		"ARAZZO_INPUT_B=from-env",
		"ARAZZO_INPUT_C=from-env",
		"ARAZZO_INPUT_D=from-env",
	}

	// Env-file provides b, c, d
	envFilePath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envFilePath, []byte("b=from-env-file\nc=from-env-file\nd=from-env-file\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Inputs-file provides c, d
	inputsFilePath := filepath.Join(tmpDir, "inputs.yaml")
	if err := os.WriteFile(inputsFilePath, []byte("c: from-inputs-file\nd: from-inputs-file\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// --inputs provides d
	flagInputs := `{"d":"from-cli-inputs"}`

	res, err := ResolveWorkflowInputs(doc, "wf", InputSources{
		Env:        env,
		EnvFile:    envFilePath,
		InputsFile: inputsFilePath,
		Inputs:     flagInputs,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Precedence check:
	// a: only in env -> from-env
	if res["a"] != "from-env" {
		t.Errorf("a: got %v, want from-env", res["a"])
	}
	// b: env-file overrides env -> from-env-file
	if res["b"] != "from-env-file" {
		t.Errorf("b: got %v, want from-env-file", res["b"])
	}
	// c: inputs-file overrides env-file -> from-inputs-file
	if res["c"] != "from-inputs-file" {
		t.Errorf("c: got %v, want from-inputs-file", res["c"])
	}
	// d: --inputs overrides inputs-file -> from-cli-inputs
	if res["d"] != "from-cli-inputs" {
		t.Errorf("d: got %v, want from-cli-inputs", res["d"])
	}
	// e: not in any source, schema default applies -> from-schema-default
	if res["e"] != "from-schema-default" {
		t.Errorf("e: got %v, want from-schema-default", res["e"])
	}
}

func TestResolveWorkflowInputsErrors(t *testing.T) {
	t.Parallel()
	doc := testWorkflowDoc(t)

	// Missing env file
	_, err := ResolveWorkflowInputs(doc, "test-wf", InputSources{
		EnvFile: "non_existent_file.env",
	})
	if err == nil || !strings.Contains(err.Error(), "non_existent_file.env") {
		t.Fatalf("expected error for missing env file, got: %v", err)
	}

	// Missing inputs file
	_, err = ResolveWorkflowInputs(doc, "test-wf", InputSources{
		InputsFile: "non_existent_inputs.yaml",
	})
	if err == nil || !strings.Contains(err.Error(), "non_existent_inputs.yaml") {
		t.Fatalf("expected error for missing inputs file, got: %v", err)
	}

	// Invalid inputs file (not an object)
	tmpDir := t.TempDir()
	badInputsPath := filepath.Join(tmpDir, "bad.yaml")
	if err := os.WriteFile(badInputsPath, []byte("[1, 2, 3]"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = ResolveWorkflowInputs(doc, "test-wf", InputSources{
		InputsFile: badInputsPath,
	})
	if err == nil || !strings.Contains(err.Error(), "mapping/object") {
		t.Fatalf("expected error for non-object inputs file, got: %v", err)
	}

	// Invalid --inputs JSON
	_, err = ResolveWorkflowInputs(doc, "test-wf", InputSources{
		Inputs: "{invalid json",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid --inputs JSON") {
		t.Fatalf("expected error for invalid --inputs JSON, got: %v", err)
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
