package runner

import (
	"encoding/json"
	"testing"

	"github.com/pb33f/libopenapi"
	libarazzo "github.com/pb33f/libopenapi/arazzo"
	"github.com/pb33f/libopenapi/arazzo/expression"
	high "github.com/pb33f/libopenapi/datamodel/high/arazzo"
	"go.yaml.in/yaml/v4"
)

func TestCompoundSimpleCriteria(t *testing.T) {
	t.Parallel()

	body := map[string]any{"challengeName": "NEW_PASSWORD_REQUIRED"}
	node := mustYAMLNode(t, body)

	condition := `$statusCode == 204 || ($statusCode == 200 && ($response.body#/challengeName == 'NEW_PASSWORD_REQUIRED' || $response.body#/challengeName == 'SOFTWARE_TOKEN_MFA'))`
	criterion := &high.Criterion{Condition: condition}

	ok, err := libarazzo.EvaluateCriterion(criterion, &expression.Context{
		StatusCode: 204,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("204 with an empty body should satisfy the login success criteria")
	}

	ok, err = libarazzo.EvaluateCriterion(criterion, &expression.Context{
		StatusCode:   200,
		ResponseBody: node,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("200 NEW_PASSWORD_REQUIRED should satisfy the login success criteria")
	}

	ok, err = libarazzo.EvaluateCriterion(criterion, &expression.Context{
		StatusCode:   200,
		ResponseBody: mustYAMLNode(t, map[string]any{"challengeName": "MFA_SETUP"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("unsupported challenge must fail the step")
	}

	ok, err = libarazzo.EvaluateCriterion(&high.Criterion{Condition: "$statusCode >= 400"}, &expression.Context{StatusCode: 401})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("401 should match abortOnAuthError")
	}
}

func TestPaginationTotalCountGreaterThanZero(t *testing.T) {
	t.Parallel()

	// json.Unmarshal uses float64 for numbers, matching the HTTP executor.
	var decoded any
	if err := json.Unmarshal([]byte(`{
		"data": [{"issue": {"id": "iss-1"}}],
		"pagination": {"count": 1, "totalCount": 1, "offset": 0, "limit": 200}
	}`), &decoded); err != nil {
		t.Fatal(err)
	}
	node := mustEncodedYAMLNode(t, decoded)

	ok, err := libarazzo.EvaluateCriterion(&high.Criterion{
		Condition: `$response.body#/pagination/totalCount > 0`,
	}, &expression.Context{
		StatusCode:   200,
		ResponseBody: node,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("totalCount 1 should satisfy > 0")
	}

	empty := mustEncodedYAMLNode(t, map[string]any{
		"data": []any{},
		"pagination": map[string]any{
			"count":      float64(0),
			"totalCount": float64(0),
			"offset":     float64(0),
			"limit":      float64(200),
		},
	})
	ok, err = libarazzo.EvaluateCriterion(&high.Criterion{
		Condition: `$response.body#/pagination/totalCount > 0`,
	}, &expression.Context{
		StatusCode:   200,
		ResponseBody: empty,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("totalCount 0 should not satisfy > 0")
	}
}

func TestArazzoYAMLPreservesGreaterThanCondition(t *testing.T) {
	t.Parallel()

	raw := []byte(`arazzo: "1.0.1"
info:
  title: t
  version: "1.0.0"
sourceDescriptions:
  - name: api
    type: openapi
    url: ./missing.yaml
workflows:
  - workflowId: w
    steps:
      - stepId: listMissionIssues
        operationId: listMissionIssues
        successCriteria:
          - condition: $statusCode == 200
          - condition: $response.body#/pagination/totalCount > 0
`)
	doc, err := libopenapi.NewArazzoDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	got := doc.Workflows[0].Steps[0].SuccessCriteria[1].Condition
	want := `$response.body#/pagination/totalCount > 0`
	if got != want {
		t.Fatalf("YAML ate the condition:\n got %q\nwant %q", got, want)
	}
}

func mustYAMLNode(t *testing.T, v any) *yaml.Node {
	t.Helper()
	b, err := yaml.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var node yaml.Node
	if err := yaml.Unmarshal(b, &node); err != nil {
		t.Fatal(err)
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return &node
}

func mustEncodedYAMLNode(t *testing.T, v any) *yaml.Node {
	t.Helper()
	node := &yaml.Node{}
	if err := node.Encode(v); err != nil {
		t.Fatal(err)
	}
	return node
}
