package company

import (
	"fmt"
	"sync"
	"time"
)

// HumanGateConfig controls when human intervention is required.
type HumanGateConfig struct {
	Enabled              bool    `yaml:"enabled"`
	TokenBudgetThreshold int64   `yaml:"token_budget_threshold"`
	RequireDeployApproval bool   `yaml:"require_deploy_approval"`
	ConfidenceFloor      float64 `yaml:"confidence_floor"`
}

// DefaultHumanGateConfig returns sensible defaults.
func DefaultHumanGateConfig() HumanGateConfig {
	return HumanGateConfig{
		Enabled:              true,
		TokenBudgetThreshold: 1000000, // 1M tokens
		RequireDeployApproval: true,
		ConfidenceFloor:      0.3,
	}
}

// HumanGateRequest represents a pending request for human intervention.
type HumanGateRequest struct {
	ID        string    `json:"id"`
	Reason    string    `json:"reason"`
	TaskID    string    `json:"taskId,omitempty"`
	WorkerID  string    `json:"workerId,omitempty"`
	Message   string    `json:"message"`
	Blocking  bool      `json:"blocking"` // true = task pauses until response
	Status    string    `json:"status"`   // pending, approved, denied
	CreatedAt time.Time `json:"createdAt"`
}

// HumanGate manages human intervention checkpoints.
type HumanGate struct {
	mu       sync.Mutex
	mgr      *Manager
	cfg      HumanGateConfig
	requests map[string]*HumanGateRequest
	idSeq    int
}

// NewHumanGate creates a new HumanGate.
func NewHumanGate(mgr *Manager, cfg HumanGateConfig) *HumanGate {
	return &HumanGate{
		mgr:      mgr,
		cfg:      cfg,
		requests: make(map[string]*HumanGateRequest),
	}
}

// CheckDeployGate returns true if deployment requires human approval and creates a request.
func (hg *HumanGate) CheckDeployGate(taskID, workerID string) *HumanGateRequest {
	if !hg.cfg.Enabled || !hg.cfg.RequireDeployApproval {
		return nil
	}

	return hg.createRequest(HumanGateRequest{
		Reason:   "deploy_approval",
		TaskID:   taskID,
		WorkerID: workerID,
		Message:  "Production deployment requires human approval",
		Blocking: true,
	})
}

// CheckBudgetGate returns a request if token consumption exceeds threshold.
func (hg *HumanGate) CheckBudgetGate(taskID string, tokensConsumed int64) *HumanGateRequest {
	if !hg.cfg.Enabled || hg.cfg.TokenBudgetThreshold <= 0 {
		return nil
	}
	if tokensConsumed < hg.cfg.TokenBudgetThreshold {
		return nil
	}

	return hg.createRequest(HumanGateRequest{
		Reason:   "budget_exceeded",
		TaskID:   taskID,
		Message:  fmt.Sprintf("Token budget exceeded: %d / %d", tokensConsumed, hg.cfg.TokenBudgetThreshold),
		Blocking: true,
	})
}

// CheckEscalationGate creates a request when circuit breaker triggers.
func (hg *HumanGate) CheckEscalationGate(taskID, workerID, reason string) *HumanGateRequest {
	if !hg.cfg.Enabled {
		return nil
	}

	return hg.createRequest(HumanGateRequest{
		Reason:   "escalation",
		TaskID:   taskID,
		WorkerID: workerID,
		Message:  fmt.Sprintf("Task escalated: %s", reason),
		Blocking: true,
	})
}

// CheckDangerousOperation creates a request for dangerous operations like force push.
func (hg *HumanGate) CheckDangerousOperation(workerID, operation string) *HumanGateRequest {
	if !hg.cfg.Enabled {
		return nil
	}

	return hg.createRequest(HumanGateRequest{
		Reason:   "dangerous_operation",
		WorkerID: workerID,
		Message:  fmt.Sprintf("Dangerous operation detected: %s", operation),
		Blocking: true,
	})
}

// RespondToRequest updates a gate request with approval or denial.
func (hg *HumanGate) RespondToRequest(requestID, status string) error {
	hg.mu.Lock()
	defer hg.mu.Unlock()

	req, ok := hg.requests[requestID]
	if !ok {
		return fmt.Errorf("gate request %q not found", requestID)
	}

	if status != "approved" && status != "denied" {
		return fmt.Errorf("invalid status %q (must be approved or denied)", status)
	}

	req.Status = status

	// When PRD is approved, trigger phase advancement
	if req.Reason == "prd_approval" && status == "approved" {
		go hg.mgr.advanceFromPRD(req.TaskID)
	}

	return nil
}

// PendingRequests returns all pending human gate requests.
func (hg *HumanGate) PendingRequests() []*HumanGateRequest {
	hg.mu.Lock()
	defer hg.mu.Unlock()

	var result []*HumanGateRequest
	for _, req := range hg.requests {
		if req.Status == "pending" {
			result = append(result, req)
		}
	}
	return result
}

// IsApproved checks if a specific request has been approved.
func (hg *HumanGate) IsApproved(requestID string) bool {
	hg.mu.Lock()
	defer hg.mu.Unlock()
	req, ok := hg.requests[requestID]
	return ok && req.Status == "approved"
}

// CreatePRDApproval creates a PRD approval gate request (used by pipeline and testing).
func (hg *HumanGate) CreatePRDApproval(taskID, workerID string) *HumanGateRequest {
	return hg.createRequest(HumanGateRequest{
		Reason:   "prd_approval",
		TaskID:   taskID,
		WorkerID: workerID,
		Message:  "PRD document ready for review",
		Blocking: true,
	})
}

func (hg *HumanGate) createRequest(req HumanGateRequest) *HumanGateRequest {
	hg.mu.Lock()
	defer hg.mu.Unlock()

	hg.idSeq++
	req.ID = fmt.Sprintf("gate-%d-%d", time.Now().UnixMilli(), hg.idSeq)
	req.Status = "pending"
	req.CreatedAt = time.Now()

	hg.requests[req.ID] = &req

	hg.mgr.emit(Event{
		Type:     EventHumanInterventionRequired,
		TaskID:   req.TaskID,
		WorkerID: req.WorkerID,
		Message:  req.Message,
	})

	return &req
}
