package company

import (
	"strings"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

func TestCreateWorkerWithTier(t *testing.T) {
	m, ch := testManager(t)

	w, err := m.CreateWorker("Consultant", "brain", WithTier(worker.TierConsultant))
	if err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}
	if w.EffectiveTier() != worker.TierConsultant {
		t.Fatalf("expected consultant, got %s", w.EffectiveTier())
	}
	drainCh(ch)
}

func TestDefaultTierIsEngineer(t *testing.T) {
	m, ch := testManager(t)

	w, err := m.CreateWorker("DefaultWorker", "robot")
	if err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}
	if w.EffectiveTier() != worker.TierEngineer {
		t.Fatalf("expected engineer default, got %s", w.EffectiveTier())
	}
	drainCh(ch)
}

func TestHierarchyValidation(t *testing.T) {
	m, ch := testManager(t)

	// Create consultant
	consultant, _ := m.CreateWorker("Boss", "crown", WithTier(worker.TierConsultant))
	drainCh(ch)

	// Create manager under consultant — should succeed
	manager, err := m.CreateWorker("Lead", "glasses",
		WithTier(worker.TierManager),
		WithParent(consultant.ID))
	if err != nil {
		t.Fatalf("expected manager under consultant to succeed: %v", err)
	}
	drainCh(ch)

	// Create engineer under manager — should succeed
	_, err = m.CreateWorker("Dev", "laptop",
		WithTier(worker.TierEngineer),
		WithParent(manager.ID))
	if err != nil {
		t.Fatalf("expected engineer under manager to succeed: %v", err)
	}
	drainCh(ch)

	// Create engineer directly under consultant — should fail
	_, err = m.CreateWorker("BadDev", "laptop",
		WithTier(worker.TierEngineer),
		WithParent(consultant.ID))
	if err == nil {
		t.Fatal("expected error: engineer cannot be under consultant")
	}

	// Create manager under manager — should fail
	_, err = m.CreateWorker("BadMgr", "glasses",
		WithTier(worker.TierManager),
		WithParent(manager.ID))
	if err == nil {
		t.Fatal("expected error: manager must be under consultant")
	}

	// Consultant with parent — should fail
	_, err = m.CreateWorker("BadConsultant", "crown",
		WithTier(worker.TierConsultant),
		WithParent(consultant.ID))
	if err == nil {
		t.Fatal("expected error: consultant cannot have parent")
	}
}

func TestGetSubordinates(t *testing.T) {
	m, ch := testManager(t)

	consultant, _ := m.CreateWorker("Boss", "crown", WithTier(worker.TierConsultant))
	drainCh(ch)

	mgr1, _ := m.CreateWorker("Lead1", "1", WithTier(worker.TierManager), WithParent(consultant.ID))
	mgr2, _ := m.CreateWorker("Lead2", "2", WithTier(worker.TierManager), WithParent(consultant.ID))
	drainCh(ch)

	subs := m.GetSubordinates(consultant.ID)
	if len(subs) != 2 {
		t.Fatalf("expected 2 subordinates, got %d", len(subs))
	}

	// Manager 1 should have no subordinates yet
	subs = m.GetSubordinates(mgr1.ID)
	if len(subs) != 0 {
		t.Fatalf("expected 0 subordinates for mgr1, got %d", len(subs))
	}

	// Add engineer under mgr2
	m.CreateWorker("Dev", "laptop", WithTier(worker.TierEngineer), WithParent(mgr2.ID))
	drainCh(ch)

	subs = m.GetSubordinates(mgr2.ID)
	if len(subs) != 1 {
		t.Fatalf("expected 1 subordinate for mgr2, got %d", len(subs))
	}
}

func TestGetManager(t *testing.T) {
	m, ch := testManager(t)

	consultant, _ := m.CreateWorker("Boss", "crown", WithTier(worker.TierConsultant))
	drainCh(ch)

	mgr, _ := m.CreateWorker("Lead", "glasses", WithTier(worker.TierManager), WithParent(consultant.ID))
	drainCh(ch)

	eng, _ := m.CreateWorker("Dev", "laptop", WithTier(worker.TierEngineer), WithParent(mgr.ID))
	drainCh(ch)

	// Engineer's manager should be the manager
	parent, ok := m.GetManager(eng.ID)
	if !ok || parent.ID != mgr.ID {
		t.Fatalf("expected engineer's manager to be %s, got %v", mgr.ID, parent)
	}

	// Manager's manager should be the consultant
	parent, ok = m.GetManager(mgr.ID)
	if !ok || parent.ID != consultant.ID {
		t.Fatalf("expected manager's parent to be %s, got %v", consultant.ID, parent)
	}

	// Consultant has no manager
	_, ok = m.GetManager(consultant.ID)
	if ok {
		t.Fatal("consultant should have no manager")
	}
}

func TestPromoteWorker(t *testing.T) {
	m, ch := testManager(t)

	w, _ := m.CreateWorker("Dev", "laptop", WithTier(worker.TierEngineer))
	drainCh(ch)

	if err := m.PromoteWorker(w.ID, worker.TierManager); err != nil {
		t.Fatalf("PromoteWorker: %v", err)
	}

	// Check event
	select {
	case e := <-ch:
		if e.Type != EventWorkerPromoted {
			t.Fatalf("expected worker_promoted, got %s", e.Type)
		}
	default:
		t.Fatal("expected promotion event")
	}

	// Verify persistence
	got, ok := m.GetWorker(w.ID)
	if !ok {
		t.Fatal("worker not found after promotion")
	}
	if got.EffectiveTier() != worker.TierManager {
		t.Fatalf("expected manager after promotion, got %s", got.EffectiveTier())
	}
}

func TestWorkerWithCLITool(t *testing.T) {
	m, ch := testManager(t)

	w, err := m.CreateWorker("AiderDev", "robot",
		WithTier(worker.TierEngineer),
		WithCLITool("aider"),
		WithBackend("ollama-codellama"))
	if err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}
	if w.CLITool != "aider" {
		t.Fatalf("expected aider, got %s", w.CLITool)
	}
	if w.BackendID != "ollama-codellama" {
		t.Fatalf("expected ollama-codellama, got %s", w.BackendID)
	}
	drainCh(ch)
}

func TestWorkerTierPersistence(t *testing.T) {
	dir := t.TempDir()
	storeDir := t.TempDir()

	store1, _ := testStoreWithDir(storeDir)
	m1, _ := New(store1, nil, nil, nil, nil, dir, nil)
	m1.CreateWorker("TierWorker", "star",
		WithTier(worker.TierManager),
		WithCLITool("claude"),
		WithBackend("claude-sonnet"))

	// Reload
	store2, _ := testStoreWithDir(storeDir)
	m2, err := New(store2, nil, nil, nil, nil, dir, nil)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	workers := m2.ListWorkers()
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}
	w := workers[0]
	if w.EffectiveTier() != worker.TierManager {
		t.Fatalf("expected manager tier after reload, got %s", w.EffectiveTier())
	}
	if w.CLITool != "claude" {
		t.Fatalf("expected claude cli after reload, got %s", w.CLITool)
	}
}

func TestReviewVerdictParsing(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"The code looks great. APPROVED", true},
		{"REJECTED: needs more tests", false},
		{"I think this is APPROVED after review", true},
		{"First I thought APPROVED but then REJECTED", false},
		{"No clear verdict in this text", false},
		{"approved by reviewer", true},
	}
	for _, tt := range tests {
		got := parseReviewVerdict(tt.output)
		if got != tt.expected {
			t.Errorf("parseReviewVerdict(%q) = %v, want %v", tt.output[:min(len(tt.output), 40)], got, tt.expected)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestReviewPipelineStartReview(t *testing.T) {
	m, ch := testManager(t)

	// Build hierarchy: consultant → manager → engineer
	consultant, _ := m.CreateWorker("Boss", "crown", WithTier(worker.TierConsultant))
	drainCh(ch)
	mgr, _ := m.CreateWorker("Lead", "glasses", WithTier(worker.TierManager), WithParent(consultant.ID))
	drainCh(ch)
	eng, _ := m.CreateWorker("Dev", "laptop", WithTier(worker.TierEngineer), WithParent(mgr.ID))
	drainCh(ch)

	// Create project and task
	p, _ := m.CreateProject("proj", "desc", "/tmp/repo", "main", nil)
	drainCh(ch)

	task := &project.Task{
		ProjectID:   p.ID,
		Title:       "Implement feature",
		Description: "Build the thing",
		Prompt:      "Implement X",
		Status:      project.TaskReady,
		BranchName:  "ai/proj/t1-feature",
		AssigneeID:  eng.ID,
	}
	m.projectStore.SaveTask(task)

	// Verify the review pipeline was initialized
	if m.review == nil {
		t.Fatal("review pipeline should be initialized")
	}

	// Verify manager relationship
	parent, ok := m.GetManager(eng.ID)
	if !ok || parent.ID != mgr.ID {
		t.Fatalf("expected engineer's manager to be %s", mgr.ID)
	}
}

func TestBuildReviewPrompt(t *testing.T) {
	task := &project.Task{
		Title:       "Fix bug",
		Description: "Fix the login bug",
		BranchName:  "ai/proj/t1-fix",
	}
	p := &project.Project{}

	m, _ := testManager(t)
	m.SetLanguage("en")
	rp := newReviewPipeline(m)
	prompt := rp.buildReviewPrompt(task, p)
	if prompt == "" {
		t.Fatal("expected non-empty review prompt")
	}
	if !strings.Contains(prompt, "ai/proj/t1-fix") {
		t.Error("prompt should contain branch name")
	}
	if !strings.Contains(prompt, "Fix bug") {
		t.Error("prompt should contain task title")
	}
	if !strings.Contains(prompt, "APPROVED") {
		t.Error("prompt should mention APPROVED verdict")
	}
}

func TestMaxWorkersEnforcement(t *testing.T) {
	m, ch := testManager(t)

	// Set max 2 engineers
	m.SetMaxWorkers(worker.TierEngineer, 2)

	_, err := m.CreateWorker("Dev1", "1")
	if err != nil {
		t.Fatalf("first engineer: %v", err)
	}
	drainCh(ch)

	_, err = m.CreateWorker("Dev2", "2")
	if err != nil {
		t.Fatalf("second engineer: %v", err)
	}
	drainCh(ch)

	// Third should fail
	_, err = m.CreateWorker("Dev3", "3")
	if err == nil {
		t.Fatal("expected error: max workers reached")
	}
	if !strings.Contains(err.Error(), "max workers") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPendingReviews(t *testing.T) {
	m, ch := testManager(t)

	reviews := m.PendingReviews()
	if len(reviews) != 0 {
		t.Fatalf("expected 0 pending reviews, got %d", len(reviews))
	}
	drainCh(ch)
}

func TestCompleteTaskWithEvents(t *testing.T) {
	m, ch := testManager(t)

	p, err := m.CreateProject("TestProj", "desc", "/tmp", "main", nil)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	drainCh(ch)

	task, err := m.AddTask(p.ID, "Task1", "desc", "prompt", nil, 1, "", "")
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	drainCh(ch)

	if err := m.CompleteTask(task.ID); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	// Check event
	select {
	case e := <-ch:
		if e.Type != EventTaskCompleted {
			t.Fatalf("expected task_completed, got %s", e.Type)
		}
	default:
		t.Fatal("expected completion event")
	}

	// Verify status
	got, ok := m.projectStore.GetTask(task.ID)
	if !ok {
		t.Fatal("task not found")
	}
	if got.Status != project.TaskDone {
		t.Fatalf("expected done, got %s", got.Status)
	}
}

func TestCompleteTaskNotFound(t *testing.T) {
	m, ch := testManager(t)
	drainCh(ch)

	err := m.CompleteTask("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func testStoreWithDir(dir string) (*project.Store, error) {
	return project.NewStore(dir)
}
