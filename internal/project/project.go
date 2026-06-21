package project

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Project represents a project entity.
type Project struct {
	ID          string
	Name        string
	Description string
	CreatedAt   string
	UpdatedAt   string
	TaskCount   int
}

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

// Service provides CRUD operations for projects.
type Service struct {
	db *sql.DB
}

// NewService creates a new project Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Create creates a new project.
func (s *Service) Create(name, description string) (*Project, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}
	if len(name) > 100 {
		return nil, fmt.Errorf("project name must not exceed 100 characters (got %d)", len(name))
	}
	if len(description) > 10000 {
		return nil, fmt.Errorf("project description must not exceed 10000 characters (got %d)", len(description))
	}

	now := time.Now().UTC().Format(time.RFC3339)
	p := &Project{
		ID:          uuid.New().String(),
		Name:        strings.TrimSpace(name),
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err := s.db.Exec(
		`INSERT INTO projects (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		// Check for unique constraint violation (case-insensitive via COLLATE NOCASE)
		if isUniqueConstraintError(err) {
			return nil, fmt.Errorf("project with name %q already exists", p.Name)
		}
		return nil, fmt.Errorf("insert project: %w", err)
	}

	return p, nil
}

// List returns all projects ordered by created_at descending, with task counts.
func (s *Service) List() ([]Project, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.name, p.description, p.created_at, p.updated_at,
		       COUNT(t.id) AS task_count
		FROM projects p
		LEFT JOIN tasks t ON t.project_id = p.id
		GROUP BY p.id
		ORDER BY p.created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt, &p.TaskCount); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return projects, nil
}

// Get returns a project by ID, including its tasks.
func (s *Service) Get(id string) (*Project, []Task, error) {
	p := &Project{}
	err := s.db.QueryRow(
		`SELECT id, name, description, created_at, updated_at FROM projects WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("project with id %q not found", id)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("query project: %w", err)
	}

	// Get associated tasks
	tasks, err := s.getTasksByProject(id)
	if err != nil {
		return nil, nil, fmt.Errorf("query tasks: %w", err)
	}

	p.TaskCount = len(tasks)
	return p, tasks, nil
}

// Update updates a project's name and/or description.
func (s *Service) Update(id, name, description string) (*Project, error) {
	// Check project exists
	existing, _, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if name != "" {
		existing.Name = strings.TrimSpace(name)
	}
	if description != "" {
		existing.Description = description
	}
	existing.UpdatedAt = now

	_, err = s.db.Exec(
		`UPDATE projects SET name = ?, description = ?, updated_at = ? WHERE id = ?`,
		existing.Name, existing.Description, existing.UpdatedAt, existing.ID,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, fmt.Errorf("project with name %q already exists", existing.Name)
		}
		return nil, fmt.Errorf("update project: %w", err)
	}

	return existing, nil
}

// Delete deletes a project and all its associated tasks in a transaction.
func (s *Service) Delete(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete associated tasks first
	if _, err := tx.Exec(`DELETE FROM tasks WHERE project_id = ?`, id); err != nil {
		return fmt.Errorf("delete tasks: %w", err)
	}

	// Delete the project
	result, err := tx.Exec(`DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("project with id %q not found", id)
	}

	return tx.Commit()
}

// getTasksByProject returns all tasks for a given project.
func (s *Service) getTasksByProject(projectID string) ([]Task, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, name, description, status, COALESCE(depends_on,''), COALESCE(fail_reason,''),
		        created_at, updated_at, COALESCE(completed_at,'')
		 FROM tasks WHERE project_id = ? ORDER BY created_at ASC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Name, &t.Description, &t.Status,
			&t.DependsOn, &t.FailReason, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// isUniqueConstraintError checks if the error is a SQLite UNIQUE constraint violation.
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed")
}
