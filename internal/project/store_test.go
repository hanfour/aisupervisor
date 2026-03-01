package project

import (
	"os"
	"testing"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestSaveAndGetProject(t *testing.T) {
	s := tempStore(t)

	p := &Project{Name: "test-proj", RepoPath: "/tmp/repo", BaseBranch: "main"}
	if err := s.SaveProject(p); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	if p.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if p.Status != ProjectActive {
		t.Fatalf("expected status active, got %s", p.Status)
	}

	got, ok := s.GetProject(p.ID)
	if !ok {
		t.Fatal("GetProject returned false")
	}
	if got.Name != "test-proj" {
		t.Fatalf("expected name test-proj, got %s", got.Name)
	}
}

func TestListProjects(t *testing.T) {
	s := tempStore(t)

	s.SaveProject(&Project{Name: "a"})
	s.SaveProject(&Project{Name: "b"})

	list := s.ListProjects()
	if len(list) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(list))
	}
}

func TestSaveAndGetTask(t *testing.T) {
	s := tempStore(t)

	task := &Task{ProjectID: "p1", Title: "do stuff", Prompt: "please do stuff"}
	if err := s.SaveTask(task); err != nil {
		t.Fatalf("SaveTask: %v", err)
	}

	if task.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if task.Status != TaskBacklog {
		t.Fatalf("expected status backlog, got %s", task.Status)
	}

	got, ok := s.GetTask(task.ID)
	if !ok {
		t.Fatal("GetTask returned false")
	}
	if got.Title != "do stuff" {
		t.Fatalf("expected title 'do stuff', got %s", got.Title)
	}
}

func TestTasksForProject(t *testing.T) {
	s := tempStore(t)

	s.SaveTask(&Task{ProjectID: "p1", Title: "a"})
	s.SaveTask(&Task{ProjectID: "p1", Title: "b"})
	s.SaveTask(&Task{ProjectID: "p2", Title: "c"})

	tasks := s.TasksForProject("p1")
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks for p1, got %d", len(tasks))
	}
}

func TestUpdateTaskStatus(t *testing.T) {
	s := tempStore(t)

	task := &Task{ProjectID: "p1", Title: "x"}
	s.SaveTask(task)

	if err := s.UpdateTaskStatus(task.ID, TaskInProgress); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}

	got, _ := s.GetTask(task.ID)
	if got.Status != TaskInProgress {
		t.Fatalf("expected in_progress, got %s", got.Status)
	}
	if got.StartedAt == nil {
		t.Fatal("expected StartedAt to be set")
	}
}

func TestUpdateTaskStatusDone(t *testing.T) {
	s := tempStore(t)

	task := &Task{ProjectID: "p1", Title: "x"}
	s.SaveTask(task)
	s.UpdateTaskStatus(task.ID, TaskDone)

	got, _ := s.GetTask(task.ID)
	if got.CompletedAt == nil {
		t.Fatal("expected CompletedAt to be set")
	}
}

func TestUpdateTaskStatusNotFound(t *testing.T) {
	s := tempStore(t)
	err := s.UpdateTaskStatus("nonexistent", TaskDone)
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestReadyTasks_NoDeps(t *testing.T) {
	s := tempStore(t)

	// Task with no dependencies should be ready immediately if backlog
	s.SaveTask(&Task{ProjectID: "p1", Title: "no deps", Status: TaskBacklog})

	ready := s.ReadyTasks("p1")
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task, got %d", len(ready))
	}
}

func TestReadyTasks_WithDeps(t *testing.T) {
	s := tempStore(t)

	t1 := &Task{ProjectID: "p1", Title: "first"}
	s.SaveTask(t1)

	t2 := &Task{ProjectID: "p1", Title: "second", DependsOn: []string{t1.ID}}
	s.SaveTask(t2)

	// t2 should not be ready because t1 is still backlog
	ready := s.ReadyTasks("p1")
	readyIDs := map[string]bool{}
	for _, r := range ready {
		readyIDs[r.ID] = true
	}
	if readyIDs[t2.ID] {
		t.Fatal("t2 should not be ready while t1 is backlog")
	}

	// Complete t1
	s.UpdateTaskStatus(t1.ID, TaskDone)

	// Now t2 should be ready
	ready = s.ReadyTasks("p1")
	readyIDs = map[string]bool{}
	for _, r := range ready {
		readyIDs[r.ID] = true
	}
	if !readyIDs[t2.ID] {
		t.Fatal("t2 should be ready after t1 is done")
	}
}

func TestPromoteReady(t *testing.T) {
	s := tempStore(t)

	t1 := &Task{ProjectID: "p1", Title: "first"}
	s.SaveTask(t1)

	t2 := &Task{ProjectID: "p1", Title: "second", DependsOn: []string{t1.ID}}
	s.SaveTask(t2)

	// Before t1 done — no promotions
	promoted, err := s.PromoteReady("p1")
	if err != nil {
		t.Fatalf("PromoteReady: %v", err)
	}
	// t1 has no deps, so it should be promoted
	hasT2 := false
	for _, p := range promoted {
		if p.ID == t2.ID {
			hasT2 = true
		}
	}
	if hasT2 {
		t.Fatal("t2 should not be promoted while t1 is backlog")
	}

	// Complete t1
	s.UpdateTaskStatus(t1.ID, TaskDone)

	promoted, err = s.PromoteReady("p1")
	if err != nil {
		t.Fatalf("PromoteReady: %v", err)
	}

	found := false
	for _, p := range promoted {
		if p.ID == t2.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("t2 should be promoted to ready")
	}

	got, _ := s.GetTask(t2.ID)
	if got.Status != TaskReady {
		t.Fatalf("expected t2 status ready, got %s", got.Status)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	// Create store, add data, close
	s1, _ := NewStore(dir)
	s1.SaveProject(&Project{Name: "persist-test", RepoPath: "/tmp"})
	s1.SaveTask(&Task{ProjectID: "p1", Title: "persist-task"})

	// Create new store from same dir — should load persisted data
	s2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore reload: %v", err)
	}

	if len(s2.ListProjects()) != 1 {
		t.Fatal("expected 1 project after reload")
	}
	if len(s2.TasksForProject("p1")) != 1 {
		t.Fatal("expected 1 task after reload")
	}
}

func TestNewStoreCreatesDir(t *testing.T) {
	dir := t.TempDir() + "/sub/dir"
	_, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore with new dir: %v", err)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("expected directory to be created")
	}
}
