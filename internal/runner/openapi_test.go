package runner

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestFromFileResolvesParentOpenAPIRefs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	parentSpec := []byte(`openapi: 3.0.3
info:
  title: parent
  version: "1"
paths: {}
components:
  schemas:
    Id:
      type: string
`)
	if err := os.WriteFile(filepath.Join(root, "openapi.yaml"), parentSpec, 0o644); err != nil {
		t.Fatal(err)
	}

	childSpec := []byte(`openapi: 3.0.3
info:
  title: child
  version: "1"
paths:
  /items/{id}:
    get:
      operationId: getItem
      parameters:
        - name: id
          in: path
          required: true
          schema:
            $ref: '../openapi.yaml#/components/schemas/Id'
      responses:
        "200":
          description: ok
`)
	if err := os.WriteFile(filepath.Join(nested, "openapi.yaml"), childSpec, 0o644); err != nil {
		t.Fatal(err)
	}

	arazzoDoc := []byte(`arazzo: 1.0.1
info:
  title: parent-ref
  version: 1.0.0
sourceDescriptions:
  - name: child
    type: openapi
    url: ./nested/openapi.yaml
workflows:
  - workflowId: read-item
    steps:
      - stepId: get
        operationId: $sourceDescriptions.child.getItem
        successCriteria:
          - condition: $statusCode == 200
`)
	arazzoPath := filepath.Join(root, "arazzo.yaml")
	if err := os.WriteFile(arazzoPath, arazzoDoc, 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := FromFile(arazzoPath, WithLogger(slog.New(slog.DiscardHandler)))
	if err != nil {
		t.Fatal(err)
	}
	list := r.ListWorkflows()
	if len(list) != 1 || list[0].WorkflowID != "read-item" {
		t.Fatalf("unexpected workflows: %+v", list)
	}
}

func TestLocalSourcePath(t *testing.T) {
	t.Parallel()

	path, ok := localSourcePath("file:///tmp/api/poi/openapi.yaml")
	if !ok || path != "/tmp/api/poi/openapi.yaml" {
		t.Fatalf("file URL: path=%q ok=%v", path, ok)
	}
	path, ok = localSourcePath("/tmp/api/poi/openapi.yaml")
	if !ok || path != "/tmp/api/poi/openapi.yaml" {
		t.Fatalf("bare path: path=%q ok=%v", path, ok)
	}
	if _, ok := localSourcePath("https://example.com/openapi.yaml"); ok {
		t.Fatal("expected remote URL to be rejected")
	}
}
