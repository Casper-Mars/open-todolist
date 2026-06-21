package server

import (
	"encoding/json"
	"net/http"

	"github.com/Casper-Mars/open-todolist/internal/task"
)

// --- Request / Response types ---

type createTaskRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	DependsOn   string `json:"depends_on"`
}

type updateTaskRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	DependsOn   *string `json:"depends_on"`
	FailReason  *string `json:"fail_reason"`
}

type setStatusRequest struct {
	Status     string `json:"status"`
	FailReason string `json:"fail_reason"`
}

type taskResponse struct {
	ID            string `json:"id"`
	ProjectID     string `json:"project_id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Status        string `json:"status"`
	DependsOn     string `json:"depends_on"`
	DependsOnName string `json:"depends_on_name"`
	FailReason    string `json:"fail_reason"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	CompletedAt   string `json:"completed_at"`
}

func toTaskResponse(t *task.Task) taskResponse {
	return taskResponse{
		ID:          t.ID,
		ProjectID:   t.ProjectID,
		Name:        t.Name,
		Description: t.Description,
		Status:      t.Status,
		DependsOn:   t.DependsOn,
		FailReason:  t.FailReason,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
		CompletedAt: t.CompletedAt,
	}
}

func toTaskWithDepsResponse(t *task.TaskWithDeps) taskResponse {
	return taskResponse{
		ID:            t.ID,
		ProjectID:     t.ProjectID,
		Name:          t.Name,
		Description:   t.Description,
		Status:        t.Status,
		DependsOn:     t.DependsOn,
		DependsOnName: t.DependsOnName,
		FailReason:    t.FailReason,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
		CompletedAt:   t.CompletedAt,
	}
}

// --- Handlers ---

// handleCreateTask handles POST /api/projects/:id/tasks
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")

	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	t, err := s.taskService.Create(projectID, req.Name, req.Description, req.DependsOn)
	if err != nil {
		writeError(w, errorToStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, toTaskResponse(t))
}

// handleListTasks handles GET /api/projects/:id/tasks
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	statusFilter := r.URL.Query().Get("status")

	tasks, err := s.taskService.List(projectID, statusFilter)
	if err != nil {
		writeError(w, errorToStatus(err), err.Error())
		return
	}

	resp := make([]taskResponse, 0, len(tasks))
	for _, t := range tasks {
		resp = append(resp, toTaskWithDepsResponse(&t))
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleGetTask handles GET /api/tasks/:id
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	t, err := s.taskService.Get(id)
	if err != nil {
		writeError(w, errorToStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toTaskWithDepsResponse(t))
}

// handleUpdateTask handles PATCH /api/tasks/:id
func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	t, err := s.taskService.Update(id, req.Name, req.Description, req.Status, req.DependsOn, req.FailReason)
	if err != nil {
		writeError(w, errorToStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toTaskWithDepsResponse(t))
}

// handleDeleteTask handles DELETE /api/tasks/:id
func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, err := s.taskService.Delete(id)
	if err != nil {
		writeError(w, errorToStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "task deleted",
		"name":          result.Name,
		"dependent_ids": result.DependentIDs,
	})
}

// handleSetTaskStatus handles PATCH /api/tasks/:id/status
func (s *Server) handleSetTaskStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req setStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	t, err := s.taskService.SetStatus(id, req.Status, req.FailReason)
	if err != nil {
		writeError(w, errorToStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toTaskWithDepsResponse(t))
}

// handleGetNextTask handles GET /api/projects/:id/tasks/next
func (s *Server) handleGetNextTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")

	tasks, err := s.taskService.GetNext(projectID)
	if err != nil {
		writeError(w, errorToStatus(err), err.Error())
		return
	}

	resp := make([]taskResponse, 0, len(tasks))
	for _, t := range tasks {
		resp = append(resp, toTaskWithDepsResponse(&t))
	}

	writeJSON(w, http.StatusOK, resp)
}
