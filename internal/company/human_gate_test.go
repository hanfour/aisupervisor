package company

import (
	"testing"
)

func TestCheckDeployGate_Enabled(t *testing.T) {
	mgr := &Manager{
		subscribers: nil,
	}
	hg := NewHumanGate(mgr, HumanGateConfig{
		Enabled:               true,
		RequireDeployApproval: true,
	}, "")

	req := hg.CheckDeployGate("t1", "w1")
	if req == nil {
		t.Fatal("expected deploy gate request")
	}
	if req.Reason != "deploy_approval" {
		t.Errorf("reason = %q, want deploy_approval", req.Reason)
	}
	if !req.Blocking {
		t.Error("deploy gate should be blocking")
	}
	if req.Status != "pending" {
		t.Errorf("status = %q, want pending", req.Status)
	}
}

func TestCheckDeployGate_Disabled(t *testing.T) {
	mgr := &Manager{}
	hg := NewHumanGate(mgr, HumanGateConfig{
		Enabled:               false,
		RequireDeployApproval: true,
	}, "")

	req := hg.CheckDeployGate("t1", "w1")
	if req != nil {
		t.Error("should not create request when disabled")
	}
}

func TestCheckBudgetGate(t *testing.T) {
	mgr := &Manager{}
	hg := NewHumanGate(mgr, HumanGateConfig{
		Enabled:              true,
		TokenBudgetThreshold: 1000,
	}, "")

	// Under threshold
	req := hg.CheckBudgetGate("t1", 500)
	if req != nil {
		t.Error("should not trigger under threshold")
	}

	// Over threshold
	req = hg.CheckBudgetGate("t1", 1500)
	if req == nil {
		t.Fatal("expected budget gate request")
	}
	if req.Reason != "budget_exceeded" {
		t.Errorf("reason = %q, want budget_exceeded", req.Reason)
	}
}

func TestRespondToRequest(t *testing.T) {
	mgr := &Manager{}
	hg := NewHumanGate(mgr, HumanGateConfig{Enabled: true, RequireDeployApproval: true}, "")

	req := hg.CheckDeployGate("t1", "w1")
	if req == nil {
		t.Fatal("expected request")
	}

	// Approve
	if err := hg.RespondToRequest(req.ID, "approved"); err != nil {
		t.Fatalf("RespondToRequest: %v", err)
	}

	if !hg.IsApproved(req.ID) {
		t.Error("request should be approved")
	}
}

func TestRespondToRequest_InvalidStatus(t *testing.T) {
	mgr := &Manager{}
	hg := NewHumanGate(mgr, HumanGateConfig{Enabled: true, RequireDeployApproval: true}, "")

	req := hg.CheckDeployGate("t1", "w1")
	err := hg.RespondToRequest(req.ID, "invalid")
	if err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestRespondToRequest_NotFound(t *testing.T) {
	mgr := &Manager{}
	hg := NewHumanGate(mgr, HumanGateConfig{Enabled: true}, "")

	err := hg.RespondToRequest("nonexistent", "approved")
	if err == nil {
		t.Error("expected error for nonexistent request")
	}
}

func TestPendingRequests(t *testing.T) {
	mgr := &Manager{}
	hg := NewHumanGate(mgr, HumanGateConfig{Enabled: true, RequireDeployApproval: true}, "")

	hg.CheckDeployGate("t1", "w1")
	hg.CheckDeployGate("t2", "w2")

	pending := hg.PendingRequests()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}

	// Approve one
	hg.RespondToRequest(pending[0].ID, "approved")
	pending = hg.PendingRequests()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending after approval, got %d", len(pending))
	}
}

func TestCheckEscalationGate(t *testing.T) {
	mgr := &Manager{}
	hg := NewHumanGate(mgr, HumanGateConfig{Enabled: true}, "")

	req := hg.CheckEscalationGate("t1", "w1", "too many bounces")
	if req == nil {
		t.Fatal("expected escalation gate request")
	}
	if req.Reason != "escalation" {
		t.Errorf("reason = %q, want escalation", req.Reason)
	}
}
