package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	projectcmd "github.com/Casper-Mars/open-todolist/cmd"
	"github.com/Casper-Mars/open-todolist/internal/database"
)

var db *database.DB
var dbPathFlag string

var version = "1.0.0"

var rootCmd = &cobra.Command{
	Use:     "otl",
	Short:   "Open Todolist - a terminal-based task manager",
	Long:    `Open Todolist (otl) is a terminal-based task management tool
that helps you organize projects and tasks with SQLite storage.`,
	Version: version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var dbPath string
		var err error

		if dbPathFlag != "" {
			dbPath = dbPathFlag
		} else {
			dbPath, err = database.DefaultPath()
			if err != nil {
				return fmt.Errorf("get default db path: %w", err)
			}
		}

		db, err = database.Open(dbPath)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}

		projectcmd.InitService(db)
		projectcmd.InitTaskService(db)
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		defer db.Close()
		dbPath, _ := database.DefaultPath()
		fmt.Printf("✓ Database initialized at %s\n", dbPath)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPathFlag, "db", "", "Database file path (default: ~/.open-todolist/data.db)")
}

func main() {
	// Register subcommands before Execute so Cobra can route to them
	projectcmd.RegisterProjectCommands(rootCmd)
	projectcmd.RegisterTaskCommands(rootCmd)
	rootCmd.AddCommand(projectcmd.ServeCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
