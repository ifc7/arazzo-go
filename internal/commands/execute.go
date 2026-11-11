package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ifc7/arazzo-go/internal/runner"
)

var (
	executeWorkflowID string
	executeInputs     string
	executeInputsFile string
	executeEnvFile    string
	executeVerbose    bool
	executeFormat     string
	executeOutput     string
)

var executeCmd = &cobra.Command{
	Use:   "execute-workflow <file>",
	Short: "Run a workflow from an Arazzo document",
	Long: `
Execute a workflow by id. Pass workflow inputs as a JSON object, from a file,
from a .env file, or via ARAZZO_INPUT_<NAME> environment variables.

Input sources merge with precedence:
  ARAZZO_INPUT_<NAME> (env) < --env-file < --inputs-file < --inputs

Environment variable and .env keys match workflow input schema properties
case-insensitively (e.g. ARAZZO_INPUT_BASEURL or ARAZZO_INPUT_baseUrl to baseUrl)
and are coerced to schema property types (integer, boolean, object as JSON).

The runner does not inject authentication. Include credentials in inputs
and rely on the cookie jar for later steps. A baseUrl input is required unless
the document already targets an absolute host.

HTTP request and response traces are off by default. Pass --verbose (or
--log-level=debug) to print them.
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format := strings.ToLower(strings.TrimSpace(executeFormat))
		if format == "" {
			format = "text"
		}
		if format != "text" && format != "json" && format != "junit" {
			return fmt.Errorf("unsupported output format %q: expected text, json, or junit", executeFormat)
		}

		r, err := loadRunner(args[0], executeVerbose)
		if err != nil {
			return err
		}

		inputs, err := r.ResolveWorkflowInputs(executeWorkflowID, runner.InputSources{
			EnvFile:    executeEnvFile,
			InputsFile: executeInputsFile,
			Inputs:     executeInputs,
			Stdin:      cmd.InOrStdin(),
		})
		if err != nil {
			return err
		}

		result, execErr := r.ExecuteWorkflow(cmd.Context(), executeWorkflowID, inputs)
		if result == nil {
			// If execution failed before producing a result, synthesize one
			// so machine-readable formats (json, junit) still emit a valid report.
			result = &runner.Result{
				WorkflowId: executeWorkflowID,
				Success:    false,
				Error:      execErr,
			}
		}

		formatted, fmtErr := formatWorkflowResult(result, format)
		if fmtErr != nil {
			return fmtErr
		}

		if strings.TrimSpace(executeOutput) != "" {
			outPath := strings.TrimSpace(executeOutput)
			if dir := filepath.Dir(outPath); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return fmt.Errorf("create output directory: %w", err)
				}
			}
			if err := os.WriteFile(outPath, []byte(formatted), 0o644); err != nil {
				return fmt.Errorf("write output file: %w", err)
			}
		} else {
			fmt.Print(formatted)
		}

		if execErr != nil {
			return fmt.Errorf("workflow failed to start: %w", execErr)
		}
		if !result.Success {
			if result.Error != nil {
				return fmt.Errorf("workflow %s failed: %w", result.WorkflowId, result.Error)
			}
			return fmt.Errorf("workflow %s failed", result.WorkflowId)
		}
		return nil
	},
}

func init() {
	executeCmd.Flags().StringVar(&executeWorkflowID, "workflow-id", "", "workflow to run")
	executeCmd.Flags().StringVar(&executeInputs, "inputs", "{}", "JSON object of workflow inputs")
	executeCmd.Flags().StringVar(&executeInputsFile, "inputs-file", "", "path to JSON/YAML inputs file, or - for stdin")
	executeCmd.Flags().StringVar(&executeEnvFile, "env-file", "", "path to .env file")
	executeCmd.Flags().BoolVarP(&executeVerbose, "verbose", "v", false, "log each HTTP request and response")
	executeCmd.Flags().StringVar(&executeFormat, "format", "text", "output format: text, json, or junit")
	executeCmd.Flags().StringVarP(&executeOutput, "output", "o", "", "write report to file instead of stdout")
	_ = executeCmd.MarkFlagRequired("workflow-id")
	rootCmd.AddCommand(executeCmd)
}
