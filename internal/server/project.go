package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Casper-Mars/open-todolist/internal/project"
	"github.com/Casper-Mars/open-todolist/internal/task"
)

// --- Request / Response types ---

type createProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type updateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type projectResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	TaskCount   int    `json:"task_count"`
}

func toProjectResponse(p *project.Project) projectResponse {
	return projectResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
		TaskCount:   p.TaskCount,
	}
}

// --- Handlers ---

// handleCreateProject handles POST /api/projects
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	p, err := s.projectService.Create(req.Name, req.Description)
	if err != nil {
		writeError(w, errorToStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, toProjectResponse(p))
}

// handleListProjects handles GET /api/projects
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.projectService.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := make([]projectResponse, 0, len(projects))
	for _, p := range projects {
		resp = append(resp, toProjectResponse(&p))
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleGetProject handles GET /api/projects/:id
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	p, tasks, err := s.projectService.Get(id)
	if err != nil {
		writeError(w, errorToStatus(err), err.Error())
		return
	}

	type projectDetailResponse struct {
		projectResponse
		Tasks []taskResponse `json:"tasks"`
	}

	taskResps := make([]taskResponse, 0, len(tasks))
	for _, pt := range tasks {
		t := task.Task{
			ID:          pt.ID,
			ProjectID:   pt.ProjectID,
			Name:        pt.Name,
			Description: pt.Description,
			Status:      pt.Status,
			DependsOn:   pt.DependsOn,
			FailReason:  pt.FailReason,
			CreatedAt:   pt.CreatedAt,
			UpdatedAt:   pt.UpdatedAt,
			CompletedAt: pt.CompletedAt,
		}
		taskResps = append(taskResps, toTaskResponse(&t))
	}

	writeJSON(w, http.StatusOK, projectDetailResponse{
		projectResponse: toProjectResponse(p),
		Tasks:           taskResps,
	})
}

// handleUpdateProject handles PATCH /api/projects/:id
func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	p, err := s.projectService.Update(id, req.Name, req.Description)
	if err != nil {
		writeError(w, errorToStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toProjectResponse(p))
}

// handleDeleteProject handles DELETE /api/projects/:id
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.projectService.Delete(id); err != nil {
		writeError(w, errorToStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "project deleted"})
}

// --- Error to HTTP status mapping ---

// errorToStatus maps service error messages to HTTP status codes.
func errorToStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	msg := err.Error()

	switch {
	case strings.Contains(msg, "already exists"):
		return http.StatusConflict
	case strings.Contains(msg, "not found"):
		return http.StatusNotFound
	case strings.Contains(msg, "invalid"):
		return http.StatusBadRequest
	case strings.Contains(msg, "cannot delete"):
		return http.StatusConflict
	case strings.Contains(msg, "cannot be empty"):
		return http.StatusBadRequest
	case strings.Contains(msg, "must not exceed"):
		return http.StatusBadRequest
	case strings.Contains(msg, "circular dependency"):
		return http.StatusBadRequest
	case strings.Contains(msg, "cannot mark"):
		return http.StatusConflict
	case strings.Contains(msg, "must be one of"):
		return http.StatusBadRequest
	case strings.Contains(msg, "is required"):
		return http.StatusBadRequest
	case strings.Contains(msg, "cannot transition"):
		return http.StatusConflict
	case strings.Contains(msg, "depend on it"):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
