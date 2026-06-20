package task

import (
	"database/sql"
	"fmt"
	"os"
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
func (s *Service) Create(projectID, name, description, dependsOn string) (*Task, error) {
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

	// Check circular dependency before setting depends_on
	if dependsOn != "" {
		if err := s.CheckCircularDependency(projectID, t.ID, dependsOn); err != nil {
			return nil, err
		}
		t.DependsOn = dependsOn
	}

	_, err = s.db.Exec(
		`INSERT INTO tasks (id, project_id, name, description, status, depends_on, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.ProjectID, t.Name, t.Description, t.Status, t.DependsOn, t.CreatedAt, t.UpdatedAt,
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

	// Resolve dependency names (query DB for names not in current list)
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
				// Dependency not in current list, query DB
				var name string
				err := s.db.QueryRow(`SELECT name FROM tasks WHERE id = ?`, t.DependsOn).Scan(&name)
				if err == nil {
					depName = name
				} else {
					depName = t.DependsOn
				}
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

		// When setting status to in_progress, check prerequisite status
		if *status == "in_progress" {
			if warning, err := s.CheckPrerequisiteStatus(id); err != nil {
				return nil, err
			} else if warning != "" {
				// Print warning but don't block
				fmt.Fprintln(os.Stderr, warning)
			}
		}
	}
	if dependsOn != nil {
		// Check circular dependency before setting depends_on
		if *dependsOn != "" {
			if err := s.CheckCircularDependency(existing.ProjectID, id, *dependsOn); err != nil {
				return nil, err
			}
		}
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

// DeleteResult contains the result of a task deletion.
type DeleteResult struct {
	Name          string
	DependentIDs  []string // IDs of tasks that depend on the deleted task
}

// Delete deletes a task by ID. Returns the task name and any dependent task IDs.
func (s *Service) Delete(id string) (*DeleteResult, error) {
	// Check if other tasks depend on this one
	deps, err := s.getDependentTasks(id)
	if err != nil {
		return nil, fmt.Errorf("check dependents: %w", err)
	}

	// Get task name before deletion
	var name string
	err = s.db.QueryRow(`SELECT name FROM tasks WHERE id = ?`, id).Scan(&name)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task with id %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query task name: %w", err)
	}

	result, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("delete task: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return nil, fmt.Errorf("task with id %q not found", id)
	}

	return &DeleteResult{Name: name, DependentIDs: deps}, nil
}

// getDependentTasks returns IDs of tasks that depend on the given task.
func (s *Service) getDependentTasks(id string) ([]string, error) {
	rows, err := s.db.Query(`SELECT id FROM tasks WHERE depends_on = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var depID string
		if err := rows.Scan(&depID); err != nil {
			return nil, err
		}
		ids = append(ids, depID)
	}
	return ids, rows.Err()
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

// maxCircularCheckDepth is the maximum depth to trace the depends_on chain
// to prevent infinite loops in case of corrupted data.
const maxCircularCheckDepth = 100

// CheckSelfDependency checks if a task depends on itself.
func CheckSelfDependency(taskID, dependsOnID string) error {
	if taskID == dependsOnID {
		return fmt.Errorf("circular dependency detected: task %q cannot depend on itself", taskID)
	}
	return nil
}

// CheckCircularDependency checks if setting taskID to depend on dependsOnID
// would create a circular dependency chain. It traces the depends_on chain
// upward from dependsOnID to see if it eventually reaches taskID.
func (s *Service) CheckCircularDependency(projectID, taskID, dependsOnID string) error {
	if dependsOnID == "" {
		return nil
	}

	// Check self-dependency first
	if err := CheckSelfDependency(taskID, dependsOnID); err != nil {
		return err
	}

	// Trace the depends_on chain upward from dependsOnID
	currentID := dependsOnID
	visited := make(map[string]bool)

	for i := 0; i < maxCircularCheckDepth; i++ {
		// If we've reached taskID, there's a cycle
		if currentID == taskID {
			return fmt.Errorf("circular dependency detected: setting depends_on would create a cycle involving task %q", taskID)
		}

		// Prevent infinite loop on self-referencing chains
		if visited[currentID] {
			return fmt.Errorf("circular dependency detected: broken dependency chain at task %q", currentID)
		}
		visited[currentID] = true

		// Look up the dependency of currentID
		var nextDependsOn string
		err := s.db.QueryRow(
			`SELECT COALESCE(depends_on, '') FROM tasks WHERE id = ? AND project_id = ?`,
			currentID, projectID,
		).Scan(&nextDependsOn)
		if err == sql.ErrNoRows {
			return fmt.Errorf("dependency task %q not found in project", currentID)
		}
		if err != nil {
			return fmt.Errorf("trace dependency chain: %w", err)
		}

		// No more dependencies — no cycle
		if nextDependsOn == "" {
			return nil
		}

		currentID = nextDependsOn
	}

	return fmt.Errorf("circular dependency check: exceeded maximum trace depth of %d", maxCircularCheckDepth)
}

// CheckPrerequisiteStatus checks if the task's dependency is completed.
// Returns a warning message if the prerequisite is not done, nil otherwise.
func (s *Service) CheckPrerequisiteStatus(taskID string) (string, error) {
	// Get the task's depends_on
	var dependsOnID string
	err := s.db.QueryRow(
		`SELECT COALESCE(depends_on, '') FROM tasks WHERE id = ?`, taskID,
	).Scan(&dependsOnID)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("task %q not found", taskID)
	}
	if err != nil {
		return "", fmt.Errorf("query task dependency: %w", err)
	}

	if dependsOnID == "" {
		return "", nil // No dependency, no warning
	}

	// Check prerequisite status
	var prereqStatus string
	var prereqName string
	err = s.db.QueryRow(
		`SELECT name, status FROM tasks WHERE id = ?`, dependsOnID,
	).Scan(&prereqName, &prereqStatus)
	if err == sql.ErrNoRows {
		return "", nil // Dependency not found (dangling), skip warning
	}
	if err != nil {
		return "", fmt.Errorf("query prerequisite task: %w", err)
	}

	if prereqStatus != "done" {
		return fmt.Sprintf("⚠ Warning: prerequisite task %q (%s) is not completed (status: %s)",
			prereqName, dependsOnID, prereqStatus), nil
	}

	return "", nil
}

// GetNext returns the next executable tasks in a project.
// Business rules:
//   - Queries all pending and failed tasks in the project
//   - Pending tasks with no dependencies → executable
//   - Pending tasks whose dependencies are done → executable
//   - Pending tasks whose dependencies are not done → skipped
//   - Failed tasks are always executable (regardless of dependency status)
//   - Results are sorted by creation time (oldest first)
func (s *Service) GetNext(projectID string) ([]TaskWithDeps, error) {
	// Verify project exists
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM projects WHERE id = ?)`, projectID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check project exists: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("project with id %q not found", projectID)
	}

	// Fetch all pending and failed tasks, ordered by creation time
	rows, err := s.db.Query(
		`SELECT id, project_id, name, description, status,
		        COALESCE(depends_on,''), COALESCE(fail_reason,''),
		        created_at, updated_at, COALESCE(completed_at,'')
		 FROM tasks
		 WHERE project_id = ? AND status IN ('pending', 'failed')
		 ORDER BY created_at ASC`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var candidates []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Name, &t.Description, &t.Status,
			&t.DependsOn, &t.FailReason, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		candidates = append(candidates, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	// Filter: determine which tasks are executable
	// Build a map of task statuses for dependency lookups
	statusMap := make(map[string]string)
	for _, t := range candidates {
		statusMap[t.ID] = t.Status
	}

	var result []TaskWithDeps
	for _, t := range candidates {
		switch t.Status {
		case "failed":
			// Failed tasks are always executable
			result = append(result, TaskWithDeps{Task: t, DependsOnName: s.resolveDepName(t.DependsOn)})
		case "pending":
			if t.DependsOn == "" {
				// No dependency → executable
				result = append(result, TaskWithDeps{Task: t, DependsOnName: ""})
			} else {
				// Check dependency status
				depStatus, inCandidates := statusMap[t.DependsOn]
				if !inCandidates {
					// Dependency not in pending/failed list, query DB
					err := s.db.QueryRow(`SELECT status FROM tasks WHERE id = ?`, t.DependsOn).Scan(&depStatus)
					if err == sql.ErrNoRows {
						// Dependency doesn't exist (dangling), treat as executable
						result = append(result, TaskWithDeps{Task: t, DependsOnName: s.resolveDepName(t.DependsOn)})
						continue
					}
					if err != nil {
						return nil, fmt.Errorf("query dependency status: %w", err)
					}
				}
				if depStatus == "done" {
					// Dependency is done → executable
					result = append(result, TaskWithDeps{Task: t, DependsOnName: s.resolveDepName(t.DependsOn)})
				}
				// Otherwise (dependency not done) → skip
			}
		}
	}

	return result, nil
}

// resolveDepName resolves a dependency ID to its task name.
func (s *Service) resolveDepName(dependsOn string) string {
	if dependsOn == "" {
		return ""
	}
	var name string
	err := s.db.QueryRow(`SELECT name FROM tasks WHERE id = ?`, dependsOn).Scan(&name)
	if err != nil {
		return dependsOn // fallback to ID if not found
	}
	return name
}

// SetStatus sets the status of a task with business rule validation.
// Rules:
//   - done tasks cannot be marked as failed
//   - already failed tasks cannot be marked as failed again
//   - fail_reason is required when status is "failed"
//   - fail_reason max length is 500 characters
//   - switching from failed to another status clears fail_reason
func (s *Service) SetStatus(id, status, failReason string) (*TaskWithDeps, error) {
	// Validate status value
	validStatuses := map[string]bool{
		"pending":     true,
		"in_progress": true,
		"done":        true,
		"failed":      true,
	}
	if !validStatuses[status] {
		return nil, fmt.Errorf("invalid status %q: must be one of pending, in_progress, done, failed", status)
	}

	// Fetch existing task
	existing, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	// Rule: done tasks cannot be marked as failed
	if existing.Status == "done" && status == "failed" {
		return nil, fmt.Errorf("cannot mark a done task as failed")
	}

	// Rule: already failed tasks cannot be marked as failed again
	if existing.Status == "failed" && status == "failed" {
		return nil, fmt.Errorf("task is already in failed status")
	}

	// Rule: fail_reason is required when status is "failed"
	if status == "failed" {
		if strings.TrimSpace(failReason) == "" {
			return nil, fmt.Errorf("fail_reason is required when setting status to failed")
		}
		if len(failReason) > 500 {
			return nil, fmt.Errorf("fail_reason must not exceed 500 characters (got %d)", len(failReason))
		}
	}

	// Rule: switching from failed to another status clears fail_reason
	if existing.Status == "failed" && status != "failed" {
		failReason = ""
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Build update
	sets := []string{"status = ?", "updated_at = ?"}
	args := []interface{}{status, now}

	// Always update fail_reason (either the provided value or empty string to clear)
	sets = append(sets, "fail_reason = ?")
	args = append(args, failReason)

	// Set completed_at when status is done
	if status == "done" {
		sets = append(sets, "completed_at = ?")
		args = append(args, now)
	}

	// When setting status to in_progress, check prerequisite status
	if status == "in_progress" {
		if warning, err := s.CheckPrerequisiteStatus(id); err != nil {
			return nil, err
		} else if warning != "" {
			fmt.Fprintln(os.Stderr, warning)
		}
	}

	args = append(args, id)
	_, err = s.db.Exec(
		fmt.Sprintf(`UPDATE tasks SET %s WHERE id = ?`, strings.Join(sets, ", ")),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("update task status: %w", err)
	}

	// Update in-memory values for the return
	existing.Status = status
	existing.FailReason = failReason
	existing.UpdatedAt = now
	if status == "done" {
		existing.CompletedAt = now
	}

	return existing, nil
}

// isUniqueConstraintError checks if the error is a SQLite UNIQUE constraint violation.
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed")
}
