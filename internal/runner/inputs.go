package runner

import (
	"fmt"
	"strings"

	higharazzo "github.com/pb33f/libopenapi/datamodel/high/arazzo"
	"go.yaml.in/yaml/v4"
)

func applyWorkflowInputDefaults(doc *higharazzo.Arazzo, workflowID string, inputs map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(inputs)+8)
	for k, v := range inputs {
		out[k] = v
	}
	if doc == nil || workflowID == "" {
		return out, nil
	}
	var wf *higharazzo.Workflow
	for _, candidate := range doc.Workflows {
		if candidate != nil && candidate.WorkflowId == workflowID {
			wf = candidate
			break
		}
	}
	if wf == nil {
		return out, nil
	}
	schema, err := workflowInputsSchema(doc, wf)
	if err != nil {
		return nil, err
	}
	applySchemaDefaults(out, schema)
	return out, nil
}

func workflowInputsSchema(doc *higharazzo.Arazzo, wf *higharazzo.Workflow) (map[string]any, error) {
	if wf == nil || wf.Inputs == nil {
		return nil, nil
	}
	decoded, err := decodeYAMLMap(wf.Inputs)
	if err != nil {
		return nil, fmt.Errorf("decode workflow %q inputs: %w", wf.WorkflowId, err)
	}
	if ref, ok := decoded["$ref"].(string); ok && ref != "" {
		return resolveComponentInputSchema(doc, ref)
	}
	return decoded, nil
}

func resolveComponentInputSchema(doc *higharazzo.Arazzo, ref string) (map[string]any, error) {
	name, ok := strings.CutPrefix(ref, "#/components/inputs/")
	if !ok || name == "" {
		return nil, fmt.Errorf("unsupported inputs $ref %q", ref)
	}
	if doc == nil || doc.Components == nil || doc.Components.Inputs == nil {
		return nil, fmt.Errorf("inputs $ref %q: document has no components.inputs", ref)
	}
	node, found := doc.Components.Inputs.Get(name)
	if !found || node == nil {
		return nil, fmt.Errorf("inputs $ref %q not found", ref)
	}
	return decodeYAMLMap(node)
}

func decodeYAMLMap(node *yaml.Node) (map[string]any, error) {
	if node == nil {
		return nil, nil
	}
	var decoded any
	if err := node.Decode(&decoded); err != nil {
		return nil, err
	}
	m, ok := decoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected mapping, got %T", decoded)
	}
	return m, nil
}

func applySchemaDefaults(inputs map[string]any, schema map[string]any) {
	if schema == nil {
		return
	}
	props, _ := schema["properties"].(map[string]any)
	for name, raw := range props {
		if _, exists := inputs[name]; exists {
			continue
		}
		prop, _ := raw.(map[string]any)
		if prop == nil {
			continue
		}
		def, has := prop["default"]
		if has {
			inputs[name] = def
		}
	}
}
