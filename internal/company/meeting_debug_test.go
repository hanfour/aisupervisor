package company

import (
	"context"
	"testing"
)

// TestDebugMeeting_ProducesFixTasks verifies that a debug meeting produces
// fix task proposals extracted from the final round.
func TestDebugMeeting_ProducesFixTasks(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewMeetingStore(dir)

	jsonFixes := `Root cause identified. Here are the fix tasks:
` + "```json" + `
[{"title":"Fix null pointer in handler","description":"Add nil check before accessing response body","priority":1},{"title":"Add regression test","description":"Cover the nil response case","priority":2}]
` + "```" + `
VOTE:approve`

	mock := &meetingMockChat{
		responses: []string{jsonFixes},
	}

	engine := NewMeetingEngine(mock, nil, nil, "en", store, &mockWorkerChecker{statuses: map[string]string{}})

	m, _ := store.Create(MeetingRequest{
		Type:         MeetingDebug,
		Title:        "Debug Crash",
		ProjectID:    "proj-1",
		TaskID:       "task-fail-1",
		ChairID:      "worker-a",
		Participants: []string{"worker-a", "worker-b"},
	})
	m.Status = MeetingInProgress
	store.Update(m)

	proposals, err := engine.RunDebugMeeting(context.Background(), m, DebugMeetingConfig{
		ErrorLog:   "panic: runtime error: invalid memory address or nil pointer dereference",
		Rejections: []string{"Review rejected: missing error handling"},
	})
	if err != nil {
		t.Fatalf("RunDebugMeeting: %v", err)
	}

	if m.Status != MeetingCompleted {
		t.Fatalf("expected status %q, got %q", MeetingCompleted, m.Status)
	}
	if len(proposals) == 0 {
		t.Fatal("expected at least 1 fix task proposal, got 0")
	}

	// Verify proposal content.
	foundFix := false
	foundTest := false
	for _, p := range proposals {
		if p.Title == "Fix null pointer in handler" {
			foundFix = true
		}
		if p.Title == "Add regression test" {
			foundTest = true
		}
	}
	if !foundFix {
		t.Fatal("expected fix proposal not found")
	}
	if !foundTest {
		t.Fatal("expected test proposal not found")
	}
}

// TestDebugMeeting_FullFlow verifies that all 3 rounds complete in a debug meeting.
func TestDebugMeeting_FullFlow(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewMeetingStore(dir)

	mock := &meetingMockChat{
		responses: []string{
			"I think the root cause is a race condition in the cache layer.\nVOTE:approve",
		},
	}

	engine := NewMeetingEngine(mock, nil, nil, "en", store, &mockWorkerChecker{statuses: map[string]string{}})

	m, _ := store.Create(MeetingRequest{
		Type:         MeetingDebug,
		Title:        "Debug Full Flow",
		ProjectID:    "proj-1",
		ChairID:      "worker-a",
		Participants: []string{"worker-a", "worker-b", "worker-c"},
	})
	m.Status = MeetingInProgress
	store.Update(m)

	proposals, err := engine.RunDebugMeeting(context.Background(), m, DebugMeetingConfig{
		ErrorLog: "timeout after 30s",
	})
	if err != nil {
		t.Fatalf("RunDebugMeeting: %v", err)
	}

	if m.Status != MeetingCompleted {
		t.Fatalf("expected status %q, got %q", MeetingCompleted, m.Status)
	}
	if len(m.Rounds) != 3 {
		t.Fatalf("expected 3 rounds, got %d", len(m.Rounds))
	}
	// No JSON proposals in the mock response, so proposals should be empty.
	if len(proposals) != 0 {
		t.Fatalf("expected 0 proposals from non-JSON response, got %d", len(proposals))
	}
	if m.Verdict == "" {
		t.Fatal("expected non-empty verdict after synthesis")
	}
	if m.CompletedAt == nil {
		t.Fatal("expected CompletedAt to be set")
	}
}
