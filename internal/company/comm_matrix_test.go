package company

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/worker"
)

func newTestManager() *Manager {
	return &Manager{
		workers: make(map[string]*worker.Worker),
	}
}

func TestCanCommunicate_ParentChild(t *testing.T) {
	mgr := newTestManager()
	mgr.workers["eng1"] = &worker.Worker{ID: "eng1", Tier: worker.TierEngineer, ParentID: "mgr1"}
	mgr.workers["mgr1"] = &worker.Worker{ID: "mgr1", Tier: worker.TierManager}

	cm := NewCommunicationMatrix(mgr)

	if !cm.CanCommunicate("eng1", "mgr1") {
		t.Error("engineer should communicate with parent manager")
	}
	if !cm.CanCommunicate("mgr1", "eng1") {
		t.Error("manager should communicate with child engineer")
	}
}

func TestCanCommunicate_SameTierSameManager(t *testing.T) {
	mgr := newTestManager()
	mgr.workers["eng1"] = &worker.Worker{ID: "eng1", Tier: worker.TierEngineer, ParentID: "mgr1"}
	mgr.workers["eng2"] = &worker.Worker{ID: "eng2", Tier: worker.TierEngineer, ParentID: "mgr1"}
	mgr.workers["mgr1"] = &worker.Worker{ID: "mgr1", Tier: worker.TierManager}

	cm := NewCommunicationMatrix(mgr)

	if !cm.CanCommunicate("eng1", "eng2") {
		t.Error("same-tier same-manager should communicate")
	}
}

func TestCanCommunicate_ConsultantManager(t *testing.T) {
	mgr := newTestManager()
	mgr.workers["con1"] = &worker.Worker{ID: "con1", Tier: worker.TierConsultant}
	mgr.workers["mgr1"] = &worker.Worker{ID: "mgr1", Tier: worker.TierManager}

	cm := NewCommunicationMatrix(mgr)

	if !cm.CanCommunicate("con1", "mgr1") {
		t.Error("consultant should communicate with manager")
	}
	if !cm.CanCommunicate("mgr1", "con1") {
		t.Error("manager should communicate with consultant")
	}
}

func TestCanCommunicate_CrossTierBlocked(t *testing.T) {
	mgr := newTestManager()
	mgr.workers["eng1"] = &worker.Worker{ID: "eng1", Tier: worker.TierEngineer, ParentID: "mgr1"}
	mgr.workers["eng2"] = &worker.Worker{ID: "eng2", Tier: worker.TierEngineer, ParentID: "mgr2"}
	mgr.workers["mgr1"] = &worker.Worker{ID: "mgr1", Tier: worker.TierManager}
	mgr.workers["mgr2"] = &worker.Worker{ID: "mgr2", Tier: worker.TierManager}

	cm := NewCommunicationMatrix(mgr)

	if cm.CanCommunicate("eng1", "eng2") {
		t.Error("engineers with different managers should not communicate directly")
	}
}

func TestCanCommunicate_NonExistentWorker(t *testing.T) {
	mgr := newTestManager()
	mgr.workers["eng1"] = &worker.Worker{ID: "eng1", Tier: worker.TierEngineer}

	cm := NewCommunicationMatrix(mgr)

	if cm.CanCommunicate("eng1", "nonexistent") {
		t.Error("should return false for non-existent worker")
	}
}

func TestRouteMessage_Direct(t *testing.T) {
	mgr := newTestManager()
	mgr.workers["eng1"] = &worker.Worker{ID: "eng1", Tier: worker.TierEngineer, ParentID: "mgr1"}
	mgr.workers["mgr1"] = &worker.Worker{ID: "mgr1", Tier: worker.TierManager}

	cm := NewCommunicationMatrix(mgr)

	route := cm.RouteMessage("eng1", "mgr1")
	if len(route) != 1 || route[0] != "mgr1" {
		t.Errorf("expected direct route [mgr1], got %v", route)
	}
}

func TestRouteMessage_ThroughManager(t *testing.T) {
	mgr := newTestManager()
	mgr.workers["eng1"] = &worker.Worker{ID: "eng1", Tier: worker.TierEngineer, ParentID: "mgr1"}
	mgr.workers["eng2"] = &worker.Worker{ID: "eng2", Tier: worker.TierEngineer, ParentID: "mgr2"}
	mgr.workers["mgr1"] = &worker.Worker{ID: "mgr1", Tier: worker.TierManager}
	mgr.workers["mgr2"] = &worker.Worker{ID: "mgr2", Tier: worker.TierManager}

	cm := NewCommunicationMatrix(mgr)

	route := cm.RouteMessage("eng1", "eng2")
	if len(route) < 2 {
		t.Errorf("expected routed path, got %v", route)
	}
	if route[0] != "mgr1" {
		t.Errorf("expected first hop to be mgr1, got %s", route[0])
	}
}
