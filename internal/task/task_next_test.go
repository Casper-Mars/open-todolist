package task

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *Service {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE TABLE tasks (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			depends_on TEXT DEFAULT '',
			fail_reason TEXT DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			completed_at TEXT DEFAULT '',
			UNIQUE(project_id, name)
		);
	`)
	if err != nil {
		t.Fatalf("create tables: %v", err)
	}

	// Insert test project
	_, err = db.Exec(`INSERT INTO projects (id, name, created_at, updated_at) VALUES ('proj-1', 'Test Project', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}

	return NewService(db)
}

// Test 1: Empty project returns empty list
func TestGetNext_EmptyProject(t *testing.T) {
	svc := setupTestDB(t)

	tasks, err := svc.GetNext("proj-1")
	if err != nil {
		t.Fatalf("GetNext: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

// Test 2: Single pending task returns that task
func TestGetNext_SinglePending(t *testing.T) {
	svc := setupTestDB(t)

	task, err := svc.Create("proj-1", "Task A", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	tasks, err := svc.GetNext("proj-1")
	if err != nil {
		t.Fatalf("GetNext: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].ID != task.ID {
		t.Errorf("expected task %s, got %s", task.ID, tasks[0].ID)
	}
}

// Test 3: A(pending) → B(pending), next only returns A (B's prerequisite not done)
func TestGetNext_ChainPending(t *testing.T) {
	svc := setupTestDB(t)

	a, err := svc.Create("proj-1", "Task A", "", "")
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}

	b, err := svc.Create("proj-1", "Task B", "", a.ID)
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}
	_ = b

	tasks, err := svc.GetNext("proj-1")
	if err != nil {
		t.Fatalf("GetNext: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].ID != a.ID {
		t.Errorf("expected task A (%s), got %s", a.ID, tasks[0].ID)
	}
}

// Test 4: A(done) → B(pending), next returns B
func TestGetNext_DependencyDone(t *testing.T) {
	svc := setupTestDB(t)

	a, err := svc.Create("proj-1", "Task A", "", "")
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}

	b, err := svc.Create("proj-1", "Task B", "", a.ID)
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}
	_ = b

	// Mark A as done (must go through in_progress first)
	_, err = svc.SetStatus(a.ID, "in_progress", "")
	if err != nil {
		t.Fatalf("SetStatus A in_progress: %v", err)
	}
	_, err = svc.SetStatus(a.ID, "done", "")
	if err != nil {
		t.Fatalf("SetStatus A done: %v", err)
	}

	tasks, err := svc.GetNext("proj-1")
	if err != nil {
		t.Fatalf("GetNext: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].ID != b.ID {
		t.Errorf("expected task B (%s), got %s", b.ID, tasks[0].ID)
	}
}

// Test 5: A(failed) → B(pending), next returns A (failed) + B not returned (prerequisite not done)
func TestGetNext_FailedDependency(t *testing.T) {
	svc := setupTestDB(t)

	a, err := svc.Create("proj-1", "Task A", "", "")
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}

	b, err := svc.Create("proj-1", "Task B", "", a.ID)
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}
	_ = b

	// Mark A as failed (must go through in_progress first)
	_, err = svc.SetStatus(a.ID, "in_progress", "")
	if err != nil {
		t.Fatalf("SetStatus A in_progress: %v", err)
	}
	_, err = svc.SetStatus(a.ID, "failed", "some error")
	if err != nil {
		t.Fatalf("SetStatus A failed: %v", err)
	}

	tasks, err := svc.GetNext("proj-1")
	if err != nil {
		t.Fatalf("GetNext: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].ID != a.ID {
		t.Errorf("expected task A (%s), got %s", a.ID, tasks[0].ID)
	}
}

// Test 6: Failed task includes fail_reason
func TestGetNext_FailedHasFailReason(t *testing.T) {
	svc := setupTestDB(t)

	a, err := svc.Create("proj-1", "Task A", "", "")
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}

	_, err = svc.SetStatus(a.ID, "in_progress", "")
	if err != nil {
		t.Fatalf("SetStatus A in_progress: %v", err)
	}
	_, err = svc.SetStatus(a.ID, "failed", "connection timeout")
	if err != nil {
		t.Fatalf("SetStatus A failed: %v", err)
	}

	tasks, err := svc.GetNext("proj-1")
	if err != nil {
		t.Fatalf("GetNext: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].FailReason != "connection timeout" {
		t.Errorf("expected fail_reason 'connection timeout', got '%s'", tasks[0].FailReason)
	}
}

// Test 7: Multiple executable tasks, all listed
func TestGetNext_MultipleExecutable(t *testing.T) {
	svc := setupTestDB(t)

	a, err := svc.Create("proj-1", "Task A", "", "")
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}
	b, err := svc.Create("proj-1", "Task B", "", "")
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}
	_ = a
	_ = b

	tasks, err := svc.GetNext("proj-1")
	if err != nil {
		t.Fatalf("GetNext: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	ids := make(map[string]bool)
	for _, t := range tasks {
		ids[t.ID] = true
	}
	if !ids[a.ID] || !ids[b.ID] {
		t.Errorf("expected both tasks A and B, got %v", ids)
	}
}

// Test: GetNext respects creation time ordering
func TestGetNext_CreationTimeOrder(t *testing.T) {
	svc := setupTestDB(t)

	// Create tasks with explicit timestamps to ensure ordering
	// Use direct SQL to control created_at precisely
	_, err := svc.db.Exec(
		`INSERT INTO tasks (id, project_id, name, description, status, depends_on, fail_reason, created_at, updated_at, completed_at)
		 VALUES ('task-b', 'proj-1', 'Task B', '', 'pending', '', '', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z', '')`)
	if err != nil {
		t.Fatalf("insert B: %v", err)
	}
	_, err = svc.db.Exec(
		`INSERT INTO tasks (id, project_id, name, description, status, depends_on, fail_reason, created_at, updated_at, completed_at)
		 VALUES ('task-a', 'proj-1', 'Task A', '', 'pending', '', '', '2024-01-02T00:00:00Z', '2024-01-02T00:00:00Z', '')`)
	if err != nil {
		t.Fatalf("insert A: %v", err)
	}

	tasks, err := svc.GetNext("proj-1")
	if err != nil {
		t.Fatalf("GetNext: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	// Task B was created first, should come first
	if tasks[0].ID != "task-b" {
		t.Errorf("expected task B first (created first), got %s", tasks[0].ID)
	}
	if tasks[1].ID != "task-a" {
		t.Errorf("expected task A second, got %s", tasks[1].ID)
	}
}

// Test: Non-existent project returns error
func TestGetNext_ProjectNotFound(t *testing.T) {
	svc := setupTestDB(t)

	_, err := svc.GetNext("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent project")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
