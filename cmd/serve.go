package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Casper-Mars/open-todolist/internal/server"
)

var (
	servePort int
	serveHost string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP API server",
	Long:  `Start the HTTP API server for Open Todolist.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := fmt.Sprintf("%s:%d", serveHost, servePort)

		srv := server.New(projectService, taskService, addr)
		if err := srv.Serve(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		return nil
	},
}

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 8080, "Port to listen on")
	serveCmd.Flags().StringVar(&serveHost, "host", "0.0.0.0", "Host to bind to")
}

// ServeCmd returns the serve subcommand for registration.
func ServeCmd() *cobra.Command {
	return serveCmd
}
