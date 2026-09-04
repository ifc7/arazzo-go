package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list-workflows <file>",
	Short: "List workflows in an Arazzo document",
	Long: `
Print each workflow id, and its summary when present.
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := loadRunner(args[0])
		if err != nil {
			return err
		}
		fmt.Print(formatWorkflowList(r.ListWorkflows()))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
