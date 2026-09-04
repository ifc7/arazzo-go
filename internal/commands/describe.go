package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var describeWorkflowID string

var describeCmd = &cobra.Command{
	Use:   "describe-workflow <file>",
	Short: "Show metadata for a workflow",
	Long: `
Print the workflow summary, description, steps, and output names.
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := loadRunner(args[0], false)
		if err != nil {
			return err
		}
		info, err := r.DescribeWorkflow(describeWorkflowID)
		if err != nil {
			return err
		}
		fmt.Print(formatWorkflowInfo(info))
		return nil
	},
}

func init() {
	describeCmd.Flags().StringVar(&describeWorkflowID, "workflow-id", "", "workflow to describe")
	_ = describeCmd.MarkFlagRequired("workflow-id")
	rootCmd.AddCommand(describeCmd)
}
