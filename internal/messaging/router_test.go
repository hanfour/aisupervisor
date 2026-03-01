package messaging

import (
	"strings"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/company"
	"github.com/hanfourmini/aisupervisor/internal/project"
)

func testCompanyMgr(t *testing.T) *company.Manager {
	t.Helper()
	dir := t.TempDir()
	store, err := project.NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	mgr, err := company.New(store, nil, nil, nil, nil, dir)
	if err != nil {
		t.Fatalf("company.New: %v", err)
	}
	return mgr
}

func TestRouterHelp(t *testing.T) {
	mgr := testCompanyMgr(t)
	r := NewRouter(mgr)
	reply := r.Handle("help")
	if !strings.Contains(reply, "Available commands") {
		t.Fatalf("expected help text, got: %s", reply)
	}
}

func TestRouterStatus(t *testing.T) {
	mgr := testCompanyMgr(t)
	r := NewRouter(mgr)
	reply := r.Handle("status")
	if !strings.Contains(reply, "Company Status") {
		t.Fatalf("expected status, got: %s", reply)
	}
	if !strings.Contains(reply, "Projects: 0") {
		t.Fatalf("expected 0 projects, got: %s", reply)
	}
}

func TestRouterProjectList(t *testing.T) {
	mgr := testCompanyMgr(t)
	mgr.CreateProject("test-project", "desc", "/tmp", "main", nil)

	r := NewRouter(mgr)
	reply := r.Handle("project list")
	if !strings.Contains(reply, "test-project") {
		t.Fatalf("expected project name, got: %s", reply)
	}
}

func TestRouterProjectCreate(t *testing.T) {
	mgr := testCompanyMgr(t)
	r := NewRouter(mgr)
	reply := r.Handle("project create My New Project")
	if !strings.Contains(reply, "Project created") {
		t.Fatalf("expected creation message, got: %s", reply)
	}

	projects := mgr.ListProjects()
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "My New Project" {
		t.Fatalf("expected name 'My New Project', got %s", projects[0].Name)
	}
}

func TestRouterWorkerList(t *testing.T) {
	mgr := testCompanyMgr(t)
	mgr.CreateWorker("Alice", "robot")

	r := NewRouter(mgr)
	reply := r.Handle("worker list")
	if !strings.Contains(reply, "Alice") {
		t.Fatalf("expected worker name, got: %s", reply)
	}
}

func TestRouterWorkerHire(t *testing.T) {
	mgr := testCompanyMgr(t)
	r := NewRouter(mgr)
	reply := r.Handle("worker hire Bob")
	if !strings.Contains(reply, "Worker hired") {
		t.Fatalf("expected hire message, got: %s", reply)
	}
}

func TestRouterUnknownCommand(t *testing.T) {
	mgr := testCompanyMgr(t)
	r := NewRouter(mgr)
	reply := r.Handle("foobar")
	if !strings.Contains(reply, "Unknown command") {
		t.Fatalf("expected unknown command message, got: %s", reply)
	}
}

func TestRouterEmptyInput(t *testing.T) {
	mgr := testCompanyMgr(t)
	r := NewRouter(mgr)
	reply := r.Handle("")
	if !strings.Contains(reply, "Available commands") {
		t.Fatalf("expected help on empty input, got: %s", reply)
	}
}
