package task

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Task represents a task entity.
type Task struct {
	ID          string
	ProjectID   string
	Name        string
	Description string
	Status      string
	DependsOn   string
	FailReason  string
	CreatedAt   string
	UpdatedAt   string
	CompletedAt string
}

// TaskWithDeps is a Task with resolved dependency names.
type TaskWithDeps struct {
	Task
	DependsOnName string
}

// Service provides CRUD operations for tasks.
type Service struct {
	db *sql.DB
}

// NewService creates a new task Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Create creates a new task in the given project.
func (s *Service) Create(projectID, name, description string) (*Task, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("task name cannot be empty")
	}
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("project ID cannot be empty")
	}

	// Verify project exists
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM projects WHERE id = ?)`, projectID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check project exists: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("project with id %q not found", projectID)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	t := &Task{
		ID:          uuid.New().String(),
		ProjectID:   projectID,
		Name:        strings.TrimSpace(name),
		Description: description,
		Status:      "pending",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err = s.db.Exec(
		`INSERT INTO tasks (id, project_id, name, description, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.ProjectID, t.Name, t.Description, t.Status, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, fmt.Errorf("task with name %q already exists in this project", t.Name)
		}
		return nil, fmt.Errorf("insert task: %w", err)
	}

	return t, nil
}

// List returns all tasks for a project, ordered by dependency (topological sort).
// If statusFilter is non-empty, only tasks with that status are returned.
func (s *Service) List(projectID, statusFilter string) ([]TaskWithDeps, error) {
	// Verify project exists
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM projects WHERE id = ?)`, projectID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check project exists: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("project with id %q not found", projectID)
	}

	// Fetch all tasks for the project
	query := `SELECT id, project_id, name, description, status,
	                 COALESCE(depends_on,''), COALESCE(fail_reason,''),
	                 created_at, updated_at, COALESCE(completed_at,'')
	          FROM tasks WHERE project_id = ?`
	args := []interface{}{projectID}

	if statusFilter != "" {
		query += ` AND status = ?`
		args = append(args, statusFilter)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Name, &t.Description, &t.Status,
			&t.DependsOn, &t.FailReason, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	// Topological sort by dependency
	sorted := topologicalSort(tasks)

	// Resolve dependency names
	taskMap := make(map[string]string)
	for _, t := range tasks {
		taskMap[t.ID] = t.Name
	}

	result := make([]TaskWithDeps, 0, len(sorted))
	for _, t := range sorted {
		depName := ""
		if t.DependsOn != "" {
			if name, ok := taskMap[t.DependsOn]; ok {
				depName = name
			} else {
				depName = t.DependsOn
			}
		}
		result = append(result, TaskWithDeps{Task: t, DependsOnName: depName})
	}

	return result, nil
}

// Get returns a task by ID with resolved dependency name.
func (s *Service) Get(id string) (*TaskWithDeps, error) {
	t := &Task{}
	err := s.db.QueryRow(
		`SELECT id, project_id, name, description, status,
		        COALESCE(depends_on,''), COALESCE(fail_reason,''),
		        created_at, updated_at, COALESCE(completed_at,'')
		 FROM tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.ProjectID, &t.Name, &t.Description, &t.Status,
		&t.DependsOn, &t.FailReason, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task with id %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query task: %w", err)
	}

	// Resolve dependency name
	depName := ""
	if t.DependsOn != "" {
		var name string
		err := s.db.QueryRow(`SELECT name FROM tasks WHERE id = ?`, t.DependsOn).Scan(&name)
		if err == nil {
			depName = name
		} else if err != sql.ErrNoRows {
			return nil, fmt.Errorf("resolve dependency: %w", err)
		} else {
			depName = t.DependsOn
		}
	}

	return &TaskWithDeps{Task: *t, DependsOnName: depName}, nil
}

// Update updates a task's fields. Only non-empty/non-zero fields are updated.
func (s *Service) Update(id string, name, description, status, dependsOn, failReason *string) (*TaskWithDeps, error) {
	// Fetch existing task
	existing, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Build dynamic update
	sets := []string{"updated_at = ?"}
	args := []interface{}{now}

	if name != nil && *name != "" {
		existing.Name = strings.TrimSpace(*name)
		sets = append(sets, "name = ?")
		args = append(args, existing.Name)
	}
	if description != nil {
		existing.Description = *description
		sets = append(sets, "description = ?")
		args = append(args, existing.Description)
	}
	if status != nil && *status != "" {
		existing.Status = *status
		sets = append(sets, "status = ?")
		args = append(args, existing.Status)

		// Set completed_at when status is done
		if *status == "done" {
			sets = append(sets, "completed_at = ?")
			args = append(args, now)
			existing.CompletedAt = now
		}
	}
	if dependsOn != nil {
		existing.DependsOn = *dependsOn
		sets = append(sets, "depends_on = ?")
		args = append(args, existing.DependsOn)
	}
	if failReason != nil {
		existing.FailReason = *failReason
		sets = append(sets, "fail_reason = ?")
		args = append(args, existing.FailReason)
	}

	existing.UpdatedAt = now

	args = append(args, id)
	_, err = s.db.Exec(
		fmt.Sprintf(`UPDATE tasks SET %s WHERE id = ?`, strings.Join(sets, ", ")),
		args...,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, fmt.Errorf("task with name %q already exists in this project", existing.Name)
		}
		return nil, fmt.Errorf("update task: %w", err)
	}

	// Re-resolve dependency name
	depName := ""
	if existing.DependsOn != "" {
		var depNameStr string
		err := s.db.QueryRow(`SELECT name FROM tasks WHERE id = ?`, existing.DependsOn).Scan(&depNameStr)
		if err == nil {
			depName = depNameStr
		} else {
			depName = existing.DependsOn
		}
	}

	return &TaskWithDeps{Task: existing.Task, DependsOnName: depName}, nil
}

// Delete deletes a task by ID.
func (s *Service) Delete(id string) error {
	result, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("task with id %q not found", id)
	}

	return nil
}

// topologicalSort returns tasks sorted so that tasks with no dependencies come first,
// and tasks that depend on others come after their dependencies.
func topologicalSort(tasks []Task) []Task {
	// Build adjacency and in-degree maps
	idSet := make(map[string]bool)
	for _, t := range tasks {
		idSet[t.ID] = true
	}

	inDegree := make(map[string]int)
	adj := make(map[string][]string)

	for _, t := range tasks {
		if _, ok := inDegree[t.ID]; !ok {
			inDegree[t.ID] = 0
		}
		if t.DependsOn != "" && idSet[t.DependsOn] {
			adj[t.DependsOn] = append(adj[t.DependsOn], t.ID)
			inDegree[t.ID]++
		}
	}

	// Start with nodes that have no dependencies
	var queue []string
	for _, t := range tasks {
		if inDegree[t.ID] == 0 {
			queue = append(queue, t.ID)
		}
	}

	// BFS topological sort
	taskMap := make(map[string]Task)
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	var sorted []Task
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		sorted = append(sorted, taskMap[id])

		for _, neighbor := range adj[id] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// Append any remaining tasks (e.g., with broken dependencies)
	for _, t := range tasks {
		found := false
		for _, s := range sorted {
			if s.ID == t.ID {
				found = true
				break
			}
		}
		if !found {
			sorted = append(sorted, t)
		}
	}

	return sorted
}

// isUniqueConstraintError checks if the error is a SQLite UNIQUE constraint violation.
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed")
}
