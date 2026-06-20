package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	projectcmd "github.com/Casper-Mars/open-todolist/cmd"
	"github.com/Casper-Mars/open-todolist/internal/database"
)

var db *database.DB

var rootCmd = &cobra.Command{
	Use:   "otl",
	Short: "Open Todolist - a terminal-based task manager",
	Long: `Open Todolist (otl) is a terminal-based task management tool
that helps you organize projects and tasks with SQLite storage.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		dbPath, err := database.DefaultPath()
		if err != nil {
			return fmt.Errorf("get default db path: %w", err)
		}

		db, err = database.Open(dbPath)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}

		projectcmd.InitService(db)
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		defer db.Close()
		dbPath, _ := database.DefaultPath()
		fmt.Printf("✓ Database initialized at %s\n", dbPath)
		return nil
	},
}

func main() {
	// Register subcommands before Execute so Cobra can route to them
	projectcmd.RegisterProjectCommands(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
