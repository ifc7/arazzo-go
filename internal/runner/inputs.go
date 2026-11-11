package runner

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	higharazzo "github.com/pb33f/libopenapi/datamodel/high/arazzo"
	"go.yaml.in/yaml/v4"

	"github.com/ifc7/arazzo-go/internal/executor"
)

// InputSources specifies sources of workflow inputs in increasing precedence:
// env (ARAZZO_INPUT_<NAME>) < env-file < inputs-file < inputs (--inputs JSON flag).
type InputSources struct {
	Env        []string  // KEY=VALUE entries; defaults to os.Environ() when nil.
	EnvFile    string    // path to .env file; ignored when empty.
	InputsFile string    // path to JSON/YAML inputs file, or "-" for Stdin; ignored when empty.
	Inputs     string    // JSON object from --inputs flag; ignored when empty or "{}".
	Stdin      io.Reader // Stdin reader used when InputsFile == "-"; defaults to os.Stdin when nil.
}

// ResolveWorkflowInputs merges inputs from env, env-file, inputs-file, and --inputs,
// coercing string types against the workflow input schema and applying schema defaults.
func (r *Runner) ResolveWorkflowInputs(workflowID string, src InputSources) (map[string]any, error) {
	if r == nil {
		return nil, fmt.Errorf("nil runner")
	}
	return ResolveWorkflowInputs(r.doc, workflowID, src)
}

// ResolveWorkflowInputs resolves workflow inputs from multiple sources in precedence order:
// env < env-file < inputs-file < --inputs, then applies workflow input schema defaults.
func ResolveWorkflowInputs(doc *higharazzo.Arazzo, workflowID string, src InputSources) (map[string]any, error) {
	var schema map[string]any
	if doc != nil && workflowID != "" {
		for _, candidate := range doc.Workflows {
			if candidate != nil && candidate.WorkflowId == workflowID {
				var err error
				schema, err = workflowInputsSchema(doc, candidate)
				if err != nil {
					return nil, err
				}
				break
			}
		}
	}
	properties, _ := schema["properties"].(map[string]any)

	result := make(map[string]any)

	// 1. Environment variables (ARAZZO_INPUT_<NAME>)
	envList := src.Env
	if envList == nil {
		envList = os.Environ()
	}
	envListCopy := append([]string(nil), envList...)
	sort.Strings(envListCopy)

	for _, entry := range envListCopy {
		k, v, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		if !strings.HasPrefix(strings.ToUpper(k), "ARAZZO_INPUT_") {
			continue
		}
		stripped := k[len("ARAZZO_INPUT_"):]
		if propName, propDef, matched := findMatchingProperty(stripped, properties); matched {
			coerced, err := coerceValue(propName, propDef, v)
			if err != nil {
				return nil, err
			}
			result[propName] = coerced
		} else if len(properties) == 0 {
			result[stripped] = v
		}
	}

	// 2. Env file (--env-file)
	if path := strings.TrimSpace(src.EnvFile); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read env file %q: %w", path, err)
		}
		envMap, err := parseEnvData(data)
		if err != nil {
			return nil, fmt.Errorf("parse env file %q: %w", path, err)
		}
		envKeys := make([]string, 0, len(envMap))
		for k := range envMap {
			envKeys = append(envKeys, k)
		}
		sort.Strings(envKeys)

		for _, k := range envKeys {
			v := envMap[k]
			target := k
			if strings.HasPrefix(strings.ToUpper(k), "ARAZZO_INPUT_") {
				target = k[len("ARAZZO_INPUT_"):]
			}
			if propName, propDef, matched := findMatchingProperty(target, properties); matched {
				coerced, err := coerceValue(propName, propDef, v)
				if err != nil {
					return nil, err
				}
				result[propName] = coerced
			} else if strings.HasPrefix(strings.ToUpper(k), "ARAZZO_INPUT_") && len(properties) == 0 {
				result[target] = v
			}
		}
	}

	// 3. Inputs file (--inputs-file <path|->)
	if path := strings.TrimSpace(src.InputsFile); path != "" {
		var data []byte
		var err error
		if path == "-" {
			r := src.Stdin
			if r == nil {
				r = os.Stdin
			}
			data, err = io.ReadAll(r)
			if err != nil {
				return nil, fmt.Errorf("read inputs from stdin: %w", err)
			}
		} else {
			data, err = os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read inputs file %q: %w", path, err)
			}
		}
		fileInputs, err := parseInputsData(data)
		if err != nil {
			return nil, fmt.Errorf("parse inputs file %q: %w", path, err)
		}
		for k, v := range fileInputs {
			key := k
			if propName, _, matched := findMatchingProperty(k, properties); matched {
				key = propName
			}
			result[key] = v
		}
	}

	// 4. CLI --inputs JSON flag
	trimmedInputs := strings.TrimSpace(src.Inputs)
	if trimmedInputs != "" && trimmedInputs != "{}" {
		var flagInputs map[string]any
		if err := json.Unmarshal([]byte(trimmedInputs), &flagInputs); err != nil {
			return nil, fmt.Errorf("invalid --inputs JSON: %w", err)
		}
		for k, v := range flagInputs {
			key := k
			if propName, _, matched := findMatchingProperty(k, properties); matched {
				key = propName
			}
			result[key] = v
		}
	}

	// 5. Schema defaults fill in remaining missing properties
	applySchemaDefaults(result, schema)

	return result, nil
}

func parseInputsData(data []byte) (map[string]any, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return map[string]any{}, nil
	}
	switch m := raw.(type) {
	case map[string]any:
		return m, nil
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, v := range m {
			out[fmt.Sprint(k)] = v
		}
		return out, nil
	default:
		return nil, fmt.Errorf("expected mapping/object, got %T", raw)
	}
}

func parseEnvData(data []byte) (map[string]string, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	result := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		line = strings.TrimSpace(line)
		key, val, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("invalid line %d: %q", lineNum, line)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				quote := val[0]
				inner := val[1 : len(val)-1]
				if quote == '"' {
					inner = unescapeDoubleQuoted(inner)
				}
				val = inner
			} else if idx := strings.Index(val, " #"); idx != -1 {
				val = strings.TrimSpace(val[:idx])
			}
		} else if idx := strings.Index(val, " #"); idx != -1 {
			val = strings.TrimSpace(val[:idx])
		}
		result[key] = val
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func unescapeDoubleQuoted(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
				i++
			case 'r':
				b.WriteByte('\r')
				i++
			case 't':
				b.WriteByte('\t')
				i++
			case '\\':
				b.WriteByte('\\')
				i++
			case '"':
				b.WriteByte('"')
				i++
			case '$':
				b.WriteByte('$')
				i++
			default:
				b.WriteByte(s[i])
			}
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func findMatchingProperty(targetKey string, properties map[string]any) (string, map[string]any, bool) {
	if len(properties) == 0 {
		return "", nil, false
	}
	// 1. Exact match
	for propName, raw := range properties {
		if propName == targetKey {
			propDef, _ := raw.(map[string]any)
			return propName, propDef, true
		}
	}
	// 2. Case-insensitive match
	for propName, raw := range properties {
		if strings.EqualFold(propName, targetKey) {
			propDef, _ := raw.(map[string]any)
			return propName, propDef, true
		}
	}
	// 3. Underscore-normalized case-insensitive match
	normalizedTarget := strings.ReplaceAll(targetKey, "_", "")
	for propName, raw := range properties {
		normalizedProp := strings.ReplaceAll(propName, "_", "")
		if strings.EqualFold(normalizedProp, normalizedTarget) {
			propDef, _ := raw.(map[string]any)
			return propName, propDef, true
		}
	}
	return "", nil, false
}

func coerceValue(propName string, propDef map[string]any, val string) (any, error) {
	if propDef == nil {
		return val, nil
	}
	var typeStr string
	switch t := propDef["type"].(type) {
	case string:
		typeStr = t
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok && s != "null" {
				typeStr = s
				break
			}
		}
	}
	typeStr = strings.ToLower(strings.TrimSpace(typeStr))

	switch typeStr {
	case "integer":
		v, err := coerceInteger(val)
		if err != nil {
			return nil, coercionError(propName, val, "integer", err)
		}
		return v, nil
	case "number":
		v, err := coerceNumber(val)
		if err != nil {
			return nil, coercionError(propName, val, "number", err)
		}
		return v, nil
	case "boolean":
		v, err := coerceBoolean(val)
		if err != nil {
			return nil, coercionError(propName, val, "boolean", err)
		}
		return v, nil
	case "object":
		v, err := coerceObject(val)
		if err != nil {
			return nil, coercionError(propName, val, "object", err)
		}
		return v, nil
	case "array":
		v, err := coerceArray(val)
		if err != nil {
			return nil, coercionError(propName, val, "array", err)
		}
		return v, nil
	default:
		return val, nil
	}
}

func coercionError(propName, val, typeName string, err error) error {
	safeVal := val
	if executor.IsSecretKey(propName) {
		safeVal = "***"
	}
	return fmt.Errorf("coerce input %q value %q to %s: %w", propName, safeVal, typeName, err)
}

func coerceInteger(val string) (any, error) {
	trimmed := strings.TrimSpace(val)
	if i, err := strconv.Atoi(trimmed); err == nil {
		return i, nil
	}
	if i64, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return i64, nil
	}
	return nil, fmt.Errorf("invalid integer")
}

func coerceNumber(val string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(val), 64)
}

func coerceBoolean(val string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "true", "t", "1", "yes", "y":
		return true, nil
	case "false", "f", "0", "no", "n":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean")
	}
}

func coerceObject(val string) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(val), &m); err != nil {
		return nil, err
	}
	if m == nil {
		return map[string]any{}, nil
	}
	return m, nil
}

func coerceArray(val string) ([]any, error) {
	var a []any
	if err := json.Unmarshal([]byte(val), &a); err != nil {
		return nil, err
	}
	if a == nil {
		return []any{}, nil
	}
	return a, nil
}

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
