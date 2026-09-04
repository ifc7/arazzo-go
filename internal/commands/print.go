package commands

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/shaunhoulihan/arazzo-go/internal/runner"
	"github.com/shaunhoulihan/arazzo-go/internal/ui"
	"go.yaml.in/yaml/v4"
)

func formatWorkflowList(workflows []runner.WorkflowInfo) string {
	var b strings.Builder
	fmt.Fprint(&b, ui.ScreenTitle("Workflows"))
	if len(workflows) == 0 {
		fmt.Fprintln(&b, "  "+ui.KeyHints("(none)"))
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, ui.KeyHints("0 workflows"))
		return b.String()
	}
	idWidth := 0
	for _, wf := range workflows {
		if n := len(wf.WorkflowID); n > idWidth {
			idWidth = n
		}
	}
	for _, wf := range workflows {
		id := ui.Apply(ui.Emphasis, fmt.Sprintf("%-*s", idWidth, wf.WorkflowID))
		if wf.Summary != "" {
			fmt.Fprintf(&b, "  %s  %s\n", id, wf.Summary)
			continue
		}
		fmt.Fprintf(&b, "  %s\n", id)
	}
	fmt.Fprintln(&b)
	label := "workflow"
	if len(workflows) != 1 {
		label = "workflows"
	}
	fmt.Fprintln(&b, ui.KeyHints(fmt.Sprintf("%d %s", len(workflows), label)))
	return b.String()
}

func formatWorkflowInfo(info *runner.WorkflowInfo) string {
	if info == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprint(&b, ui.ScreenTitle(info.WorkflowID))
	if info.Summary != "" {
		fmt.Fprintln(&b, "  "+ui.Field("summary", info.Summary))
	}
	if desc := strings.TrimSpace(info.Description); desc != "" {
		fmt.Fprintln(&b, "  "+ui.Section("description"))
		for _, line := range strings.Split(desc, "\n") {
			fmt.Fprintln(&b, "    "+line)
		}
	}
	if len(info.OutputNames) > 0 {
		fmt.Fprintln(&b, "  "+ui.Field("outputs", strings.Join(info.OutputNames, ", ")))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, ui.Section("steps"))
	if len(info.Steps) == 0 {
		fmt.Fprintln(&b, "  "+ui.KeyHints("(none)"))
	}
	numWidth := len(fmt.Sprintf("%d", len(info.Steps)))
	for i, step := range info.Steps {
		if i > 0 {
			fmt.Fprintln(&b)
		}
		target := step.OperationID
		if target == "" {
			target = step.OperationPath
		}
		if target == "" {
			target = step.WorkflowID
		}
		fmt.Fprintf(&b, "  %s  %s\n", ui.KeyHints(fmt.Sprintf("%*d", numWidth, i+1)), ui.Apply(ui.Emphasis, step.StepID))
		if target != "" {
			fmt.Fprintf(&b, "     %s\n", ui.KeyHints(target))
		}
		if desc := strings.TrimSpace(step.Description); desc != "" {
			for _, line := range strings.Split(desc, "\n") {
				fmt.Fprintf(&b, "     %s\n", line)
			}
		}
	}
	return b.String()
}

func formatResult(result *runner.Result) string {
	if result == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprint(&b, ui.ScreenTitle("Workflow "+result.WorkflowId))
	status := ui.Apply(ui.Success, "success")
	if !result.Success {
		status = ui.Apply(ui.Error, "failed")
	}
	fmt.Fprintln(&b, "  "+ui.Field("status", status))
	fmt.Fprintln(&b, "  "+ui.Field("duration", result.Duration.String()))
	fmt.Fprintln(&b, "  "+ui.Field("steps", fmt.Sprintf("%d", len(result.Steps))))
	if len(result.Outputs) > 0 {
		fmt.Fprintln(&b, "  "+ui.Field("outputs", formatJSON(result.Outputs)))
	}
	if result.Error != nil {
		fmt.Fprintln(&b, "  "+ui.Field("error", ui.Apply(ui.Error, result.Error.Error())))
	}
	if len(result.Steps) == 0 {
		return b.String()
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, ui.Section("steps"))
	for _, step := range result.Steps {
		if step == nil {
			continue
		}
		mark := ui.Apply(ui.Success, "✓")
		if !step.Success {
			mark = ui.Apply(ui.Error, "✗")
		}
		fmt.Fprintf(&b, "  %s  %s  %s\n", mark, ui.Apply(ui.Emphasis, step.StepId), ui.KeyHints(step.Duration.String()))
		if !step.Success && step.Error != nil {
			fmt.Fprintf(&b, "     %s\n", ui.Apply(ui.Error, step.Error.Error()))
		}
		if len(step.Outputs) > 0 {
			fmt.Fprintf(&b, "     %s %s\n", ui.KeyHints("outputs:"), formatJSON(step.Outputs))
		}
	}
	return b.String()
}

func formatJSON(v any) string {
	b, err := json.Marshal(jsonable(v))
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func jsonable(v any) any {
	switch t := v.(type) {
	case *yaml.Node:
		return yamlNodeJSON(t)
	case yaml.Node:
		return yamlNodeJSON(&t)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = jsonable(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = jsonable(val)
		}
		return out
	default:
		return v
	}
}

func yamlNodeJSON(node *yaml.Node) any {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return yamlNodeJSON(node.Content[0])
	}
	switch node.Kind {
	case yaml.MappingNode:
		out := make(map[string]any, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			out[node.Content[i].Value] = yamlNodeJSON(node.Content[i+1])
		}
		return out
	case yaml.SequenceNode:
		out := make([]any, len(node.Content))
		for i, child := range node.Content {
			out[i] = yamlNodeJSON(child)
		}
		return out
	case yaml.ScalarNode:
		switch node.Tag {
		case "!!int":
			if v, err := strconv.ParseInt(node.Value, 10, 64); err == nil {
				return v
			}
		case "!!float":
			if v, err := strconv.ParseFloat(node.Value, 64); err == nil {
				return v
			}
		case "!!bool":
			if v, err := strconv.ParseBool(node.Value); err == nil {
				return v
			}
		case "!!null":
			return nil
		}
		return node.Value
	default:
		return node.Value
	}
}
