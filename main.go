package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Casper-Mars/open-todolist/internal/database"
)

var rootCmd = &cobra.Command{
	Use:   "otl",
	Short: "Open Todolist - a terminal-based task manager",
	Long: `Open Todolist (otl) is a terminal-based task management tool
that helps you organize projects and tasks with SQLite storage.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, err := database.DefaultPath()
		if err != nil {
			return fmt.Errorf("get default db path: %w", err)
		}

		db, err := database.Open(dbPath)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		fmt.Printf("✓ Database initialized at %s\n", dbPath)
		return nil
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
