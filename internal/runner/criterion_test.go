package runner

import (
	"testing"

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
