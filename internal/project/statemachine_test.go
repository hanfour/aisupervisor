package project

import (
	"testing"
	"time"
)

func TestCanTransition_ValidTransitions(t *testing.T) {
	validCases := []struct {
		from, to TaskStatus
	}{
		{TaskBacklog, TaskDraft},
		{TaskBacklog, TaskReady},
		{TaskDraft, TaskSpecReview},
		{TaskDraft, TaskReady},
		{TaskSpecReview, TaskApproved},
		{TaskSpecReview, TaskRevision},
		{TaskApproved, TaskReady},
		{TaskReady, TaskAssigned},
		{TaskAssigned, TaskInProgress},
		{TaskAssigned, TaskReady},
		{TaskInProgress, TaskCodeReview},
		{TaskInProgress, TaskReview},
		{TaskInProgress, TaskDone},
		{TaskInProgress, TaskFailed},
		{TaskCodeReview, TaskTesting},
		{TaskCodeReview, TaskDone},
		{TaskCodeReview, TaskRevision},
		{TaskReview, TaskDone},
		{TaskReview, TaskRevision},
		{TaskTesting, TaskSecurityScan},
		{TaskTesting, TaskRevision},
		{TaskSecurityScan, TaskStaging},
		{TaskSecurityScan, TaskRevision},
		{TaskStaging, TaskAccepted},
		{TaskStaging, TaskRevision},
		{TaskAccepted, TaskDone},
		{TaskAccepted, TaskDeployed},
		{TaskRevision, TaskInProgress},
		{TaskRevision, TaskReady},
		{TaskRevision, TaskFailed},
		{TaskDone, TaskDeployed},
		{TaskFailed, TaskReady},
		{TaskFailed, TaskBacklog},
		{TaskEscalation, TaskReady},
		{TaskEscalation, TaskBacklog},
		{TaskEscalation, TaskFailed},
	}

	for _, tc := range validCases {
		if !CanTransition(tc.from, tc.to) {
			t.Errorf("expected valid transition %s → %s", tc.from, tc.to)
		}
	}
}

func TestCanTransition_InvalidTransitions(t *testing.T) {
	invalidCases := []struct {
		from, to TaskStatus
	}{
		{TaskBacklog, TaskInProgress},
		{TaskReady, TaskDone},
		{TaskInProgress, TaskDeployed},
		{TaskDone, TaskBacklog},
		{TaskAssigned, TaskDone},
		{TaskTesting, TaskDone},
		{TaskDeployed, TaskBacklog},
	}

	for _, tc := range invalidCases {
		if CanTransition(tc.from, tc.to) {
			t.Errorf("expected invalid transition %s → %s", tc.from, tc.to)
		}
	}
}

func TestValidateTransition_ReturnsError(t *testing.T) {
	err := ValidateTransition(TaskBacklog, TaskInProgress)
	if err == nil {
		t.Error("expected error for invalid transition")
	}

	err = ValidateTransition(TaskBacklog, TaskReady)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNormalizeStatus(t *testing.T) {
	if NormalizeStatus("review") != TaskReview {
		t.Error("expected 'review' to normalize to TaskReview")
	}
	if NormalizeStatus("in_progress") != TaskInProgress {
		t.Error("expected 'in_progress' to remain unchanged")
	}
}

func TestShouldEscalate(t *testing.T) {
	task := &Task{RejectionCount: 2}
	if ShouldEscalate(task) {
		t.Error("should not escalate at 2 rejections")
	}

	task.RejectionCount = 3
	if !ShouldEscalate(task) {
		t.Error("should escalate at 3 rejections")
	}
}

func TestStageForRole(t *testing.T) {
	cases := map[string]TaskStatus{
		"architect": TaskSpecReview,
		"coder":     TaskInProgress,
		"qa":        TaskTesting,
		"security":  TaskSecurityScan,
		"devops":    TaskStaging,
		"unknown":   TaskInProgress,
	}

	for role, expected := range cases {
		stages := StageForRole(role)
		if len(stages) != 1 || stages[0] != expected {
			t.Errorf("StageForRole(%q) = %v, want [%s]", role, stages, expected)
		}
	}
}

func TestBackwardCompatFlow(t *testing.T) {
	// Test the legacy flow: backlog → ready → assigned → in_progress → review → done
	flow := []TaskStatus{TaskBacklog, TaskReady, TaskAssigned, TaskInProgress, TaskReview, TaskDone}
	for i := 0; i < len(flow)-1; i++ {
		if !CanTransition(flow[i], flow[i+1]) {
			t.Errorf("backward compat flow broken at %s → %s", flow[i], flow[i+1])
		}
	}
}

func TestFullLifecycleFlow(t *testing.T) {
	// Test the full new flow: backlog → draft → spec_review → approved → ready → assigned → in_progress → code_review → testing → security_scan → staging → accepted → deployed
	flow := []TaskStatus{
		TaskBacklog, TaskDraft, TaskSpecReview, TaskApproved, TaskReady,
		TaskAssigned, TaskInProgress, TaskCodeReview, TaskTesting,
		TaskSecurityScan, TaskStaging, TaskAccepted, TaskDeployed,
	}
	for i := 0; i < len(flow)-1; i++ {
		if !CanTransition(flow[i], flow[i+1]) {
			t.Errorf("full lifecycle flow broken at %s → %s", flow[i], flow[i+1])
		}
	}
}

func TestRejectionStruct(t *testing.T) {
	r := Rejection{
		Stage:      TaskCodeReview,
		RejectorID: "mgr-1",
		Reason:     "missing tests",
		Timestamp:  time.Now(),
	}
	if r.Stage != TaskCodeReview {
		t.Error("wrong stage")
	}
}
