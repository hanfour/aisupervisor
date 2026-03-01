package company

import (
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/project"
)

func testStore(t *testing.T) *project.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := project.NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func testManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	store := testStore(t)

	// Create manager without spawner/gitops/monitor (nil) for unit tests
	m, err := New(store, nil, nil, nil, nil, dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

func TestCreateProject(t *testing.T) {
	m := testManager(t)

	p, err := m.CreateProject("test", "desc", "/tmp/repo", "main", []string{"goal1"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if p.Name != "test" {
		t.Fatalf("expected name 'test', got %s", p.Name)
	}

	// Verify event emitted
	select {
	case e := <-m.events:
		if e.Type != EventProjectCreated {
			t.Fatalf("expected project_created event, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected event but none received")
	}
}

func TestListProjects(t *testing.T) {
	m := testManager(t)

	m.CreateProject("a", "", "/tmp/a", "main", nil)
	m.CreateProject("b", "", "/tmp/b", "main", nil)
	// Drain events
	drainEvents(m.events)

	list := m.ListProjects()
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}
}

func TestAddTask(t *testing.T) {
	m := testManager(t)

	p, _ := m.CreateProject("proj", "", "/tmp", "main", nil)
	drainEvents(m.events)

	task, err := m.AddTask(p.ID, "do thing", "desc", "prompt text", nil, 1, "v1")
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	if task.ID == "" {
		t.Fatal("expected auto-generated task ID")
	}
	if task.Status != project.TaskReady {
		t.Fatalf("expected ready (no deps), got %s", task.Status)
	}
	if task.BranchName == "" {
		t.Fatal("expected branch name to be set")
	}

	select {
	case e := <-m.events:
		if e.Type != EventTaskCreated {
			t.Fatalf("expected task_created, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected event")
	}
}

func TestAddTaskWithDeps(t *testing.T) {
	m := testManager(t)

	p, _ := m.CreateProject("proj", "", "/tmp", "main", nil)
	drainEvents(m.events)

	t1, _ := m.AddTask(p.ID, "first", "", "prompt1", nil, 1, "")
	drainEvents(m.events)

	t2, err := m.AddTask(p.ID, "second", "", "prompt2", []string{t1.ID}, 2, "")
	if err != nil {
		t.Fatalf("AddTask with deps: %v", err)
	}
	// Task with deps should default to backlog
	if t2.Status != project.TaskBacklog {
		t.Fatalf("expected backlog for task with deps, got %s", t2.Status)
	}
}

func TestAddTaskProjectNotFound(t *testing.T) {
	m := testManager(t)
	_, err := m.AddTask("nonexistent", "task", "", "prompt", nil, 1, "")
	if err == nil {
		t.Fatal("expected error for nonexistent project")
	}
}

func TestCreateWorker(t *testing.T) {
	m := testManager(t)

	w, err := m.CreateWorker("Alice", "robot")
	if err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}
	if w.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if w.Name != "Alice" {
		t.Fatalf("expected Alice, got %s", w.Name)
	}

	select {
	case e := <-m.events:
		if e.Type != EventWorkerSpawned {
			t.Fatalf("expected worker_spawned, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected event")
	}
}

func TestListWorkers(t *testing.T) {
	m := testManager(t)

	m.CreateWorker("A", "robot")
	m.CreateWorker("B", "kirby")
	drainEvents(m.events)

	workers := m.ListWorkers()
	if len(workers) != 2 {
		t.Fatalf("expected 2 workers, got %d", len(workers))
	}
}

func TestWorkerPersistence(t *testing.T) {
	dir := t.TempDir()
	storeDir := t.TempDir()
	store, _ := project.NewStore(storeDir)

	m1, _ := New(store, nil, nil, nil, nil, dir)
	m1.CreateWorker("Persist", "mario")
	drainEvents(m1.events)

	// Reload
	m2, err := New(store, nil, nil, nil, nil, dir)
	if err != nil {
		t.Fatalf("reload manager: %v", err)
	}

	workers := m2.ListWorkers()
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker after reload, got %d", len(workers))
	}
	if workers[0].Name != "Persist" {
		t.Fatalf("expected name 'Persist', got %s", workers[0].Name)
	}
}

func TestProjectProgress(t *testing.T) {
	m := testManager(t)

	p, _ := m.CreateProject("prog", "", "/tmp", "main", nil)
	drainEvents(m.events)

	m.AddTask(p.ID, "t1", "", "p1", nil, 1, "")
	m.AddTask(p.ID, "t2", "", "p2", nil, 1, "")
	m.AddTask(p.ID, "t3", "", "p3", nil, 1, "")
	drainEvents(m.events)

	tasks := m.ListTasks(p.ID)
	// Complete first task
	m.CompleteTask(tasks[0].ID)
	drainEvents(m.events)

	prog := m.ProjectProgress(p.ID)
	if prog.Total != 3 {
		t.Fatalf("expected total 3, got %d", prog.Total)
	}
	if prog.Done != 1 {
		t.Fatalf("expected 1 done, got %d", prog.Done)
	}
	if prog.Percent < 33.0 || prog.Percent > 34.0 {
		t.Fatalf("expected ~33%%, got %.1f%%", prog.Percent)
	}
}

func TestCompleteTask(t *testing.T) {
	m := testManager(t)

	p, _ := m.CreateProject("p", "", "/tmp", "main", nil)
	drainEvents(m.events)

	t1, _ := m.AddTask(p.ID, "first", "", "p1", nil, 1, "")
	t2, _ := m.AddTask(p.ID, "second", "", "p2", []string{t1.ID}, 2, "")
	drainEvents(m.events)

	// Complete t1 — should promote t2 to ready
	if err := m.CompleteTask(t1.ID); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	// Check t2 is now ready
	got, _ := m.projectStore.GetTask(t2.ID)
	if got.Status != project.TaskReady {
		t.Fatalf("expected t2 ready after t1 done, got %s", got.Status)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"Add Login Page", "add-login-page"},
		{"fix: bug #123", "fix-bug-123"},
		{"  spaces  ", "spaces"},
		{"UPPER", "upper"},
	}
	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.expected {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func drainEvents(ch chan Event) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}
