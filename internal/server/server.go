package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Casper-Mars/open-todolist/internal/project"
	"github.com/Casper-Mars/open-todolist/internal/task"
)

// Server is the HTTP API server.
type Server struct {
	projectService *project.Service
	taskService    *task.Service
	addr           string
}

// New creates a new Server.
func New(projectService *project.Service, taskService *task.Service, addr string) *Server {
	return &Server{
		projectService: projectService,
		taskService:    taskService,
		addr:           addr,
	}
}

// registerRoutes registers all HTTP routes on the given mux.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Health check (no middleware)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Project endpoints
	mux.HandleFunc("POST /api/projects", s.handleCreateProject)
	mux.HandleFunc("GET /api/projects", s.handleListProjects)
	mux.HandleFunc("GET /api/projects/{id}", s.handleGetProject)
	mux.HandleFunc("PATCH /api/projects/{id}", s.handleUpdateProject)
	mux.HandleFunc("DELETE /api/projects/{id}", s.handleDeleteProject)

	// Task endpoints
	mux.HandleFunc("POST /api/projects/{id}/tasks", s.handleCreateTask)
	mux.HandleFunc("GET /api/projects/{id}/tasks", s.handleListTasks)
	mux.HandleFunc("GET /api/tasks/{id}", s.handleGetTask)
	mux.HandleFunc("PATCH /api/tasks/{id}", s.handleUpdateTask)
	mux.HandleFunc("DELETE /api/tasks/{id}", s.handleDeleteTask)
	mux.HandleFunc("PATCH /api/tasks/{id}/status", s.handleSetTaskStatus)
	mux.HandleFunc("GET /api/projects/{id}/tasks/next", s.handleGetNextTask)
}

// Serve starts the HTTP server and blocks until shutdown.
func (s *Server) Serve() error {
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	// Apply middleware to all non-health routes
	handler := chain(mux, limitBody, requireJSON)

	httpServer := &http.Server{
		Addr:    s.addr,
		Handler: handler,
	}

	// Try to listen first to detect port conflicts early
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	fmt.Printf("Server listening on http://%s\n", s.addr)

	// Graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Error: server error: %v\n", err)
			os.Exit(1)
		}
	}()

	<-quit
	fmt.Println("\nShutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	fmt.Println("Server stopped")
	return nil
}
