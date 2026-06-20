package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Casper-Mars/open-todolist/internal/database"
	"github.com/Casper-Mars/open-todolist/internal/task"
)

// taskService is set by InitTaskService after DB is opened.
var taskService *task.Service

// InitTaskService initializes the task service with an open database connection.
func InitTaskService(db *database.DB) {
	taskService = task.NewService(db.DB)
}

// RegisterTaskCommands registers the task subcommands on the root command.
func RegisterTaskCommands(root *cobra.Command) {
	root.AddCommand(taskCmd)
	taskCmd.AddCommand(taskCreateCmd)
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskShowCmd)
	taskCmd.AddCommand(taskUpdateCmd)
	taskCmd.AddCommand(taskDeleteCmd)
	taskCmd.AddCommand(taskStatusCmd)
	taskCmd.AddCommand(taskNextCmd)
}

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks",
	Long:  `Create, list, show, update, and delete tasks within a project.`,
}

// --- create ---

var taskCreateCmd = &cobra.Command{
	Use:   "create <project-id> <name>",
	Short: "Create a new task",
	Long:  `Create a new task in the specified project. Task names must be unique within a project (case-insensitive).`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]
		name := args[1]
		description, _ := cmd.Flags().GetString("description")
		dependsOn, _ := cmd.Flags().GetString("depends_on")

		t, err := taskService.Create(projectID, name, description, dependsOn)
		if err != nil {
			return err
		}

		fmt.Printf("✓ Task created\n")
		fmt.Printf("  ID:          %s\n", t.ID)
		fmt.Printf("  Project:     %s\n", t.ProjectID)
		fmt.Printf("  Name:        %s\n", t.Name)
		if t.Description != "" {
			fmt.Printf("  Description: %s\n", t.Description)
		}
		if t.DependsOn != "" {
			fmt.Printf("  Depends On:  %s\n", t.DependsOn)
		}
		fmt.Printf("  Status:      %s\n", t.Status)
		fmt.Printf("  Created:     %s\n", t.CreatedAt)
		return nil
	},
}

// --- list ---

var taskListCmd = &cobra.Command{
	Use:   "list <project-id>",
	Short: "List tasks in a project",
	Long: `List all tasks in a project, ordered by dependency (topological sort).
Tasks with no dependencies are listed first; tasks that depend on others
are listed after their dependencies.

Use --status to filter by status (pending, in_progress, done, failed).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]
		status, _ := cmd.Flags().GetString("status")

		tasks, err := taskService.List(projectID, status)
		if err != nil {
			return err
		}

		if len(tasks) == 0 {
			fmt.Println("No tasks found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tSTATUS\tDEPENDS ON\tCREATED")
		for _, t := range tasks {
			dep := t.DependsOnName
			if dep == "" {
				dep = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", t.ID, t.Name, t.Status, dep, t.CreatedAt)
		}
		w.Flush()
		return nil
	},
}

// --- show ---

var taskShowCmd = &cobra.Command{
	Use:   "show <task-id>",
	Short: "Show task details",
	Long:  `Show full task information including dependency name and failure reason.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		t, err := taskService.Get(id)
		if err != nil {
			return err
		}

		fmt.Printf("Task: %s\n", t.Name)
		fmt.Printf("  ID:          %s\n", t.ID)
		fmt.Printf("  Project:     %s\n", t.ProjectID)
		fmt.Printf("  Description: %s\n", t.Description)
		fmt.Printf("  Status:      %s\n", t.Status)
		if t.DependsOn != "" {
			fmt.Printf("  Depends On:  %s (%s)\n", t.DependsOnName, t.DependsOn)
		} else {
			fmt.Printf("  Depends On:  -\n")
		}
		if t.FailReason != "" {
			fmt.Printf("  Fail Reason: %s\n", t.FailReason)
		}
		fmt.Printf("  Created:     %s\n", t.CreatedAt)
		fmt.Printf("  Updated:     %s\n", t.UpdatedAt)
		if t.CompletedAt != "" {
			fmt.Printf("  Completed:   %s\n", t.CompletedAt)
		}
		return nil
	},
}

// --- update ---

var taskUpdateCmd = &cobra.Command{
	Use:   "update <task-id>",
	Short: "Update a task",
	Long: `Update a task's fields. At least one flag must be provided.

Supported flags:
  --name         New task name
  --description  New task description
  --status       New status (pending, in_progress, done, failed)
  --depends_on   Task ID this task depends on (use empty string to clear)
  --fail_reason  Failure reason (only meaningful when status is failed)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		var name, description, status, dependsOn, failReason *string

		if cmd.Flags().Changed("name") {
			v, _ := cmd.Flags().GetString("name")
			name = &v
		}
		if cmd.Flags().Changed("description") {
			v, _ := cmd.Flags().GetString("description")
			description = &v
		}
		if cmd.Flags().Changed("status") {
			v, _ := cmd.Flags().GetString("status")
			status = &v
		}
		if cmd.Flags().Changed("depends_on") {
			v, _ := cmd.Flags().GetString("depends_on")
			dependsOn = &v
		}
		if cmd.Flags().Changed("fail_reason") {
			v, _ := cmd.Flags().GetString("fail_reason")
			failReason = &v
		}

		if name == nil && description == nil && status == nil && dependsOn == nil && failReason == nil {
			return fmt.Errorf("at least one flag must be provided (--name, --description, --status, --depends_on, --fail_reason)")
		}

		t, err := taskService.Update(id, name, description, status, dependsOn, failReason)
		if err != nil {
			return err
		}

		fmt.Printf("✓ Task updated\n")
		fmt.Printf("  ID:          %s\n", t.ID)
		fmt.Printf("  Name:        %s\n", t.Name)
		fmt.Printf("  Description: %s\n", t.Description)
		fmt.Printf("  Status:      %s\n", t.Status)
		if t.DependsOn != "" {
			fmt.Printf("  Depends On:  %s\n", t.DependsOnName)
		}
		if t.FailReason != "" {
			fmt.Printf("  Fail Reason: %s\n", t.FailReason)
		}
		fmt.Printf("  Updated:     %s\n", t.UpdatedAt)
		return nil
	},
}

// --- delete ---

var taskDeleteCmd = &cobra.Command{
	Use:   "delete <task-id>",
	Short: "Delete a task",
	Long:  `Delete a task. Requires confirmation unless --force is used.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		force, _ := cmd.Flags().GetBool("force")

		// Show task info before deletion
		t, err := taskService.Get(id)
		if err != nil {
			return err
		}

		fmt.Printf("Task: %s\n", t.Name)
		fmt.Printf("  ID:          %s\n", t.ID)
		fmt.Printf("  Status:      %s\n", t.Status)

		if !force {
			fmt.Printf("\nAre you sure you want to delete this task? (y/N): ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("read input: %w", err)
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "y" && input != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		result, err := taskService.Delete(id)
		if err != nil {
			return err
		}

		fmt.Printf("✓ Task %q deleted\n", result.Name)
		if len(result.DependentIDs) > 0 {
			fmt.Printf("⚠ Warning: %d task(s) depended on this task and now have a dangling dependency:\n", len(result.DependentIDs))
			for _, depID := range result.DependentIDs {
				fmt.Printf("  - %s\n", depID)
			}
		}
		return nil
	},
}

func init() {
	taskCreateCmd.Flags().StringP("description", "d", "", "Task description")
	taskCreateCmd.Flags().String("depends_on", "", "Task ID this task depends on")

	taskListCmd.Flags().String("status", "", "Filter by status (pending, in_progress, done, failed)")

	taskUpdateCmd.Flags().StringP("name", "n", "", "New task name")
	taskUpdateCmd.Flags().StringP("description", "d", "", "New task description")
	taskUpdateCmd.Flags().String("status", "", "New status (pending, in_progress, done, failed)")
	taskUpdateCmd.Flags().String("depends_on", "", "Task ID this task depends on")
	taskUpdateCmd.Flags().String("fail_reason", "", "Failure reason")

	taskDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}
