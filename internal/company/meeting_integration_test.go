package company

import (
	"context"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Integration tests — end-to-end meeting flows using mock ChatProvider
// ---------------------------------------------------------------------------

func TestMeetingIntegration_ReviewFlow(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMeetingStore(dir)
	if err != nil {
		t.Fatalf("NewMeetingStore: %v", err)
	}
	mailbox, err := NewMailbox(dir)
	if err != nil {
		t.Fatalf("NewMailbox: %v", err)
	}

	// Mock chat: all participants approve, plus synthesis response.
	// RunReviewMeeting does up to 3 rounds (3 participants each = 9 calls),
	// then Synthesize may call chat once more if no consensus from rounds.
	// With unanimous approve at threshold 0.67, round 1 reaches consensus
	// immediately (3/3 = 1.0 >= 0.67), so only 3 chat calls for round 1
	// plus 0 for synthesis (consensus path doesn't call AI).
	mock := &meetingMockChat{
		responses: []string{
			"The code is well-structured and follows conventions.\nVOTE:approve",
		},
	}

	wc := &mockWorkerChecker{
		statuses: map[string]string{
			"alice":   "idle",
			"bob":     "idle",
			"charlie": "idle",
		},
	}

	engine := NewMeetingEngine(mock, mailbox, nil, "en", store, wc)

	// 1. Schedule
	m, err := engine.Schedule(MeetingRequest{
		Type:         MeetingReview,
		Title:        "Integration Review Test",
		ProjectID:    "proj-1",
		TaskID:       "task-42",
		ChairID:      "alice",
		Participants: []string{"alice", "bob", "charlie"},
		Agenda:       []string{"review PR #42"},
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	if m.Status != MeetingScheduled {
		t.Fatalf("expected scheduled, got %q", m.Status)
	}

	// 2. Start
	err = engine.Start(context.Background(), m.ID)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Re-fetch after Start mutated the store copy.
	m, _ = store.Get(m.ID)
	if m.Status != MeetingInProgress {
		t.Fatalf("expected in_progress after Start, got %q", m.Status)
	}

	// 3. Run review meeting
	cfg := ReviewMeetingConfig{Diff: "--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-old\n+new"}
	err = engine.RunReviewMeeting(context.Background(), m, cfg)
	if err != nil {
		t.Fatalf("RunReviewMeeting: %v", err)
	}

	// 4. Verify final state
	final, _ := store.Get(m.ID)
	if final.Status != MeetingCompleted {
		t.Fatalf("expected completed, got %q", final.Status)
	}
	if final.Verdict == "" {
		t.Fatal("expected non-empty verdict")
	}
	if final.Verdict != "approve" {
		t.Fatalf("expected verdict 'approve', got %q", final.Verdict)
	}
	if len(final.Rounds) < 1 {
		t.Fatal("expected at least 1 round")
	}
	if final.Summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if final.CompletedAt == nil {
		t.Fatal("expected CompletedAt to be set")
	}
}

func TestMeetingIntegration_PlanningProducesTasks(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMeetingStore(dir)
	if err != nil {
		t.Fatalf("NewMeetingStore: %v", err)
	}

	// Mock returns a response containing task proposals as JSON + a vote.
	// Planning runs 3 rounds; proposals are extracted from round 3.
	mock := &meetingMockChat{
		responses: []string{
			"Here is my plan:\n```json\n[{\"title\":\"Setup database\",\"description\":\"Create schema\"},{\"title\":\"Build API\",\"description\":\"REST endpoints\"}]\n```\nVOTE:approve",
		},
	}

	wc := &mockWorkerChecker{
		statuses: map[string]string{
			"lead":  "idle",
			"dev-a": "idle",
			"dev-b": "idle",
		},
	}

	engine := NewMeetingEngine(mock, nil, nil, "en", store, wc)

	// Schedule + Start
	m, err := engine.Schedule(MeetingRequest{
		Type:         MeetingPlanning,
		Title:        "Sprint Planning",
		ProjectID:    "proj-2",
		ChairID:      "lead",
		Participants: []string{"lead", "dev-a", "dev-b"},
		Agenda:       []string{"plan next sprint"},
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	err = engine.Start(context.Background(), m.ID)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	m, _ = store.Get(m.ID)

	// Run planning meeting
	cfg := PlanningMeetingConfig{
		Goals:       []string{"deliver MVP"},
		Constraints: []string{"2-week sprint"},
	}
	proposals, err := engine.RunPlanningMeeting(context.Background(), m, cfg)
	if err != nil {
		t.Fatalf("RunPlanningMeeting: %v", err)
	}

	// Verify proposals
	if len(proposals) == 0 {
		t.Fatal("expected at least 1 task proposal")
	}

	// Check that known titles appear (all 3 participants return the same response,
	// but extractTaskProposals deduplicates by title).
	foundDB := false
	foundAPI := false
	for _, p := range proposals {
		if p.Title == "Setup database" {
			foundDB = true
			if p.Description != "Create schema" {
				t.Fatalf("unexpected description for 'Setup database': %q", p.Description)
			}
		}
		if p.Title == "Build API" {
			foundAPI = true
		}
	}
	if !foundDB {
		t.Fatal("expected proposal 'Setup database'")
	}
	if !foundAPI {
		t.Fatal("expected proposal 'Build API'")
	}

	// Verify meeting completed
	final, _ := store.Get(m.ID)
	if final.Status != MeetingCompleted {
		t.Fatalf("expected completed, got %q", final.Status)
	}
	if len(final.Rounds) != 3 {
		t.Fatalf("expected 3 rounds, got %d", len(final.Rounds))
	}
}

func TestMeetingIntegration_DebugFlow(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMeetingStore(dir)
	if err != nil {
		t.Fatalf("NewMeetingStore: %v", err)
	}

	// Mock returns debugging analysis with fix proposals.
	mock := &meetingMockChat{
		responses: []string{
			"Root cause: missing null check in handler.\n```json\n[{\"title\":\"Add null check\",\"description\":\"Fix NPE in request handler\"}]\n```\nVOTE:approve",
		},
	}

	wc := &mockWorkerChecker{
		statuses: map[string]string{
			"senior": "idle",
			"dev-x":  "idle",
		},
	}

	engine := NewMeetingEngine(mock, nil, nil, "en", store, wc)

	m, err := engine.Schedule(MeetingRequest{
		Type:         MeetingDebug,
		Title:        "Debug NPE",
		ProjectID:    "proj-3",
		TaskID:       "task-99",
		ChairID:      "senior",
		Participants: []string{"senior", "dev-x"},
		Agenda:       []string{"diagnose null pointer exception"},
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	err = engine.Start(context.Background(), m.ID)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	m, _ = store.Get(m.ID)

	cfg := DebugMeetingConfig{
		ErrorLog:   "panic: runtime error: invalid memory address or nil pointer dereference",
		Rejections: []string{"previous fix attempt failed"},
	}
	proposals, err := engine.RunDebugMeeting(context.Background(), m, cfg)
	if err != nil {
		t.Fatalf("RunDebugMeeting: %v", err)
	}

	// Verify proposals
	if len(proposals) == 0 {
		t.Fatal("expected at least 1 fix proposal")
	}

	foundFix := false
	for _, p := range proposals {
		if p.Title == "Add null check" {
			foundFix = true
		}
	}
	if !foundFix {
		t.Fatal("expected proposal 'Add null check'")
	}

	// Verify meeting completed
	final, _ := store.Get(m.ID)
	if final.Status != MeetingCompleted {
		t.Fatalf("expected completed, got %q", final.Status)
	}
	if len(final.Rounds) != 3 {
		t.Fatalf("expected 3 rounds for debug meeting, got %d", len(final.Rounds))
	}
}

func TestMeetingIntegration_CancelMidway(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMeetingStore(dir)
	if err != nil {
		t.Fatalf("NewMeetingStore: %v", err)
	}
	mailbox, err := NewMailbox(dir)
	if err != nil {
		t.Fatalf("NewMailbox: %v", err)
	}

	wc := &mockWorkerChecker{statuses: map[string]string{}}
	engine := NewMeetingEngine(nil, mailbox, nil, "en", store, wc)

	// 1. Schedule
	m, err := engine.Schedule(MeetingRequest{
		Type:         MeetingReview,
		Title:        "Cancel Test Meeting",
		ProjectID:    "proj-1",
		ChairID:      "chair",
		Participants: []string{"worker-a", "worker-b"},
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	if m.Status != MeetingScheduled {
		t.Fatalf("expected scheduled, got %q", m.Status)
	}

	// Drain schedule notifications.
	mailbox.Deliver("worker-a")
	mailbox.Deliver("worker-b")

	// 2. Cancel before starting
	err = engine.Cancel(m.ID)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	// 3. Verify status is cancelled
	final, err := store.Get(m.ID)
	if err != nil {
		t.Fatalf("Get after cancel: %v", err)
	}
	if final.Status != MeetingCancelled {
		t.Fatalf("expected cancelled, got %q", final.Status)
	}

	// Verify cancel notifications were sent
	msgsA := mailbox.Peek("worker-a")
	if len(msgsA) != 1 {
		t.Fatalf("expected 1 cancel notification for worker-a, got %d", len(msgsA))
	}
	if !strings.Contains(msgsA[0].Content, "cancelled") {
		t.Fatalf("expected cancel notification content, got %q", msgsA[0].Content)
	}
}

func TestMeetingIntegration_StartWithBusyWorker(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMeetingStore(dir)
	if err != nil {
		t.Fatalf("NewMeetingStore: %v", err)
	}

	wc := &mockWorkerChecker{
		statuses: map[string]string{
			"alice":   "idle",
			"bob":     "working", // busy
			"charlie": "idle",
		},
	}

	engine := NewMeetingEngine(nil, nil, nil, "en", store, wc)

	// 1. Schedule with 3 participants
	m, err := engine.Schedule(MeetingRequest{
		Type:         MeetingReview,
		Title:        "Busy Worker Test",
		ProjectID:    "proj-1",
		ChairID:      "alice",
		Participants: []string{"alice", "bob", "charlie"},
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	// 2. Start should fail because bob is busy
	err = engine.Start(context.Background(), m.ID)
	if err == nil {
		t.Fatal("expected error when starting with busy worker")
	}

	// 3. Error should mention the busy worker
	if !strings.Contains(err.Error(), "bob") {
		t.Fatalf("error should mention busy worker 'bob', got: %v", err)
	}

	// Meeting should remain scheduled
	final, _ := store.Get(m.ID)
	if final.Status != MeetingScheduled {
		t.Fatalf("expected status to remain scheduled, got %q", final.Status)
	}
}

func TestMeetingIntegration_PersistAndReload(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMeetingStore(dir)
	if err != nil {
		t.Fatalf("NewMeetingStore: %v", err)
	}

	// Use a mock that returns approve votes.
	mock := &meetingMockChat{
		responses: []string{"Analysis complete.\nVOTE:approve"},
	}

	wc := &mockWorkerChecker{
		statuses: map[string]string{
			"worker-a": "idle",
			"worker-b": "idle",
		},
	}

	engine := NewMeetingEngine(mock, nil, nil, "en", store, wc)

	// 1. Schedule meeting
	m, err := engine.Schedule(MeetingRequest{
		Type:         MeetingReview,
		Title:        "Persist Test Meeting",
		ProjectID:    "proj-persist",
		ChairID:      "worker-a",
		Participants: []string{"worker-a", "worker-b"},
		Agenda:       []string{"test persistence"},
		MaxRounds:    5,
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	// Start the meeting.
	err = engine.Start(context.Background(), m.ID)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	m, _ = store.Get(m.ID)

	// 2. Run 1 round manually (not the full meeting flow)
	_, _, err = engine.RunRound(context.Background(), m, 1, ExecAPI, 0.67)
	if err != nil {
		t.Fatalf("RunRound: %v", err)
	}

	// 3. Save store (already saved by RunRound, but explicit save to be sure)
	err = store.Save()
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 4. Create a new store from the same directory
	store2, err := NewMeetingStore(dir)
	if err != nil {
		t.Fatalf("NewMeetingStore reload: %v", err)
	}

	// 5. Verify meeting data persisted
	reloaded, err := store2.Get(m.ID)
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}

	if reloaded.Title != "Persist Test Meeting" {
		t.Fatalf("title mismatch: expected %q, got %q", "Persist Test Meeting", reloaded.Title)
	}
	if reloaded.ProjectID != "proj-persist" {
		t.Fatalf("projectID mismatch: expected %q, got %q", "proj-persist", reloaded.ProjectID)
	}
	if reloaded.MaxRounds != 5 {
		t.Fatalf("MaxRounds mismatch: expected 5, got %d", reloaded.MaxRounds)
	}
	if reloaded.Status != MeetingInProgress {
		t.Fatalf("status mismatch: expected %q, got %q", MeetingInProgress, reloaded.Status)
	}

	// Verify round data persisted
	if len(reloaded.Rounds) != 1 {
		t.Fatalf("expected 1 round after reload, got %d", len(reloaded.Rounds))
	}
	if reloaded.Rounds[0].Number != 1 {
		t.Fatalf("expected round number 1, got %d", reloaded.Rounds[0].Number)
	}
	if len(reloaded.Rounds[0].Speeches) != 2 {
		t.Fatalf("expected 2 speeches in round 1, got %d", len(reloaded.Rounds[0].Speeches))
	}

	// Verify speech data
	for _, speech := range reloaded.Rounds[0].Speeches {
		if speech.Vote != "approve" {
			t.Fatalf("expected vote 'approve' after reload, got %q", speech.Vote)
		}
		if speech.Content == "" {
			t.Fatal("expected non-empty speech content after reload")
		}
	}
}
