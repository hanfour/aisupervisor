package company

import (
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
	m1, _ := New(store1, nil, nil, nil, nil, dir)
	m1.CreateWorker("TierWorker", "star",
		WithTier(worker.TierManager),
		WithCLITool("claude"),
		WithBackend("claude-sonnet"))

	// Reload
	store2, _ := testStoreWithDir(storeDir)
	m2, err := New(store2, nil, nil, nil, nil, dir)
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

func testStoreWithDir(dir string) (*project.Store, error) {
	return project.NewStore(dir)
}
