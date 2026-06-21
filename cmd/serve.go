package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
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

		mux := http.NewServeMux()

		// Health check endpoint
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		})

		server := &http.Server{
			Addr:    addr,
			Handler: mux,
		}

		// Try to listen first to detect port conflicts early
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to start server: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Server listening on http://%s\n", addr)

		// Graceful shutdown on SIGINT/SIGTERM
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "Error: server error: %v\n", err)
				os.Exit(1)
			}
		}()

		<-quit
		fmt.Println("\nShutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: server forced to shutdown: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Server stopped")
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
