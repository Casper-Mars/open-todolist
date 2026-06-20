package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var taskStatusCmd = &cobra.Command{
	Use:   "status <task-id> <status>",
	Short: "Set task status",
	Long: `Set the status of a task.

Supported statuses: pending, in_progress, done, failed

When setting status to "failed", --reason is required and must not exceed 500 characters.
Switching from "failed" to another status automatically clears the fail_reason.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		status := args[1]
		reason, _ := cmd.Flags().GetString("reason")

		t, err := taskService.SetStatus(id, status, reason)
		if err != nil {
			return err
		}

		fmt.Printf("✓ Task status updated\n")
		fmt.Printf("  ID:          %s\n", t.ID)
		fmt.Printf("  Name:        %s\n", t.Name)
		fmt.Printf("  Status:      %s\n", t.Status)
		if t.FailReason != "" {
			fmt.Printf("  Fail Reason: %s\n", t.FailReason)
		}
		fmt.Printf("  Updated:     %s\n", t.UpdatedAt)
		return nil
	},
}

func init() {
	taskStatusCmd.Flags().StringP("reason", "r", "", "Failure reason (required when status is failed, max 500 chars)")
}
