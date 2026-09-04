package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var (
	executeWorkflowID string
	executeInputs     string
)

var executeCmd = &cobra.Command{
	Use:   "execute-workflow <file>",
	Short: "Run a workflow from an Arazzo document",
	Long: `
Execute a workflow by id. Pass workflow inputs as a JSON object.

The runner does not inject authentication. Include credentials in --inputs
and rely on the cookie jar for later steps. A baseUrl input (or --inputs
baseUrl) is required unless the document already targets an absolute host.
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputs := map[string]any{}
		if strings.TrimSpace(executeInputs) != "" {
			if err := json.Unmarshal([]byte(executeInputs), &inputs); err != nil {
				return fmt.Errorf("invalid --inputs JSON: %w", err)
			}
		}

		r, err := loadRunner(args[0])
		if err != nil {
			return err
		}

		result, err := r.ExecuteWorkflow(cmd.Context(), executeWorkflowID, inputs)
		if err != nil {
			return fmt.Errorf("workflow failed to start: %w", err)
		}
		fmt.Print(formatResult(result))
		if !result.Success {
			return fmt.Errorf("workflow %s failed", result.WorkflowId)
		}
		return nil
	},
}

func init() {
	executeCmd.Flags().StringVar(&executeWorkflowID, "workflow-id", "", "workflow to run")
	executeCmd.Flags().StringVar(&executeInputs, "inputs", "{}", "JSON object of workflow inputs")
	_ = executeCmd.MarkFlagRequired("workflow-id")
	rootCmd.AddCommand(executeCmd)
}
