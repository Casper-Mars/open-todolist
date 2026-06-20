package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Casper-Mars/open-todolist/internal/database"
	"github.com/Casper-Mars/open-todolist/internal/project"
)

// projectService is set by InitService after DB is opened.
var projectService *project.Service

// InitService initializes the project service with an open database connection.
func InitService(db *database.DB) {
	projectService = project.NewService(db.DB)
}

// RegisterProjectCommands registers the project subcommands on the root command.
func RegisterProjectCommands(root *cobra.Command) {
	root.AddCommand(projectCmd)
	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectShowCmd)
	projectCmd.AddCommand(projectUpdateCmd)
	projectCmd.AddCommand(projectDeleteCmd)
}

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
	Long:  `Create, list, show, update, and delete projects.`,
}

// --- create ---

var projectCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new project",
	Long:  `Create a new project with a unique name.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		description, _ := cmd.Flags().GetString("description")

		p, err := projectService.Create(name, description)
		if err != nil {
			return err
		}

		fmt.Printf("✓ Project created\n")
		fmt.Printf("  ID:          %s\n", p.ID)
		fmt.Printf("  Name:        %s\n", p.Name)
		if p.Description != "" {
			fmt.Printf("  Description: %s\n", p.Description)
		}
		fmt.Printf("  Created:     %s\n", p.CreatedAt)
		return nil
	},
}

// --- list ---

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	Long:  `List all projects ordered by creation time (newest first), with task counts.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projects, err := projectService.List()
		if err != nil {
			return err
		}

		if len(projects) == 0 {
			fmt.Println("No projects found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tTASKS\tCREATED")
		for _, p := range projects {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", p.ID, p.Name, p.TaskCount, p.CreatedAt)
		}
		w.Flush()
		return nil
	},
}

// --- show ---

var projectShowCmd = &cobra.Command{
	Use:   "show <project-id>",
	Short: "Show project details",
	Long:  `Show full project information and associated tasks.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		p, tasks, err := projectService.Get(id)
		if err != nil {
			return err
		}

		fmt.Printf("Project: %s\n", p.Name)
		fmt.Printf("  ID:          %s\n", p.ID)
		fmt.Printf("  Description: %s\n", p.Description)
		fmt.Printf("  Created:     %s\n", p.CreatedAt)
		fmt.Printf("  Updated:     %s\n", p.UpdatedAt)
		fmt.Printf("  Tasks:       %d\n", p.TaskCount)

		if len(tasks) > 0 {
			fmt.Println()
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  ID\tNAME\tSTATUS\tCREATED")
			for _, t := range tasks {
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", t.ID, t.Name, t.Status, t.CreatedAt)
			}
			w.Flush()
		}
		return nil
	},
}

// --- update ---

var projectUpdateCmd = &cobra.Command{
	Use:   "update <project-id>",
	Short: "Update a project",
	Long:  `Update a project's name and/or description.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		name, _ := cmd.Flags().GetString("name")
		description, _ := cmd.Flags().GetString("description")

		if name == "" && description == "" {
			return fmt.Errorf("at least one of --name or --description must be provided")
		}

		p, err := projectService.Update(id, name, description)
		if err != nil {
			return err
		}

		fmt.Printf("✓ Project updated\n")
		fmt.Printf("  ID:          %s\n", p.ID)
		fmt.Printf("  Name:        %s\n", p.Name)
		fmt.Printf("  Description: %s\n", p.Description)
		fmt.Printf("  Updated:     %s\n", p.UpdatedAt)
		return nil
	},
}

// --- delete ---

var projectDeleteCmd = &cobra.Command{
	Use:   "delete <project-id>",
	Short: "Delete a project",
	Long:  `Delete a project and all its associated tasks. Requires confirmation unless --force is used.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		force, _ := cmd.Flags().GetBool("force")

		// Show project info before deletion
		p, tasks, err := projectService.Get(id)
		if err != nil {
			return err
		}

		fmt.Printf("Project: %s (%d tasks)\n", p.Name, len(tasks))
		fmt.Printf("  ID:          %s\n", p.ID)
		fmt.Printf("  Description: %s\n", p.Description)

		if !force {
			fmt.Printf("\nAre you sure you want to delete this project and all its tasks? (y/N): ")
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

		if err := projectService.Delete(id); err != nil {
			return err
		}

		fmt.Printf("✓ Project %q deleted\n", p.Name)
		return nil
	},
}

func init() {
	projectCreateCmd.Flags().StringP("description", "d", "", "Project description")
	projectUpdateCmd.Flags().StringP("name", "n", "", "New project name")
	projectUpdateCmd.Flags().StringP("description", "d", "", "New project description")
	projectDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}
