package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var taskNextCmd = &cobra.Command{
	Use:   "next <project-id>",
	Short: "Show next executable tasks",
	Long: `Show the next executable tasks in a project.

Business rules:
  - Pending tasks with no dependencies are executable
  - Pending tasks whose dependencies are done are executable
  - Pending tasks whose dependencies are not done are skipped
  - Failed tasks are always executable (regardless of dependencies)
  - Results are sorted by creation time (oldest first)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]

		tasks, err := taskService.GetNext(projectID)
		if err != nil {
			return err
		}

		if len(tasks) == 0 {
			fmt.Println("无待执行任务")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tSTATUS\tDEPENDS ON\tFAIL REASON\tCREATED")
		for _, t := range tasks {
			dep := t.DependsOnName
			if dep == "" {
				dep = "-"
			}
			failReason := t.FailReason
			if failReason == "" {
				failReason = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				t.ID, t.Name, t.Status, dep, failReason, t.CreatedAt)
		}
		w.Flush()
		return nil
	},
}
