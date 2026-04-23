package company

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockWorkerChecker implements workerChecker for tests.
type mockWorkerChecker struct {
	statuses map[string]string
}

func (m *mockWorkerChecker) GetWorkerStatus(id string) (string, bool) {
	s, ok := m.statuses[id]
	return s, ok
}

// ---------------------------------------------------------------------------
// MeetingStore tests
// ---------------------------------------------------------------------------

func TestMeetingStore_NewEmpty(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMeetingStore(dir)
	if err != nil {
		t.Fatalf("NewMeetingStore: %v", err)
	}
	if got := len(store.List()); got != 0 {
		t.Fatalf("expected 0 meetings, got %d", got)
	}

	// Verify meetings/ directory was created.
	info, err := os.Stat(filepath.Join(dir, "meetings"))
	if err != nil {
		t.Fatalf("meetings dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("meetings is not a directory")
	}
}

func TestMeetingStore_CreateAndGet(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMeetingStore(dir)
	if err != nil {
		t.Fatalf("NewMeetingStore: %v", err)
	}

	req := MeetingRequest{
		Type:         MeetingReview,
		Title:        "Code Review #1",
		ProjectID:    "proj-1",
		TaskID:       "task-1",
		ChairID:      "worker-a",
		Participants: []string{"worker-a", "worker-b", "worker-c"},
		Agenda:       []string{"review PR", "discuss design"},
		MaxRounds:    5,
	}

	created, err := store.Create(req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if created.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if created.Status != MeetingScheduled {
		t.Fatalf("expected status %q, got %q", MeetingScheduled, created.Status)
	}
	if created.MaxRounds != 5 {
		t.Fatalf("expected MaxRounds 5, got %d", created.MaxRounds)
	}
	if created.Title != "Code Review #1" {
		t.Fatalf("expected title %q, got %q", "Code Review #1", created.Title)
	}
	if created.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}

	// Get by ID.
	fetched, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.ID != created.ID {
		t.Fatalf("ID mismatch: %q vs %q", fetched.ID, created.ID)
	}
	if fetched.Title != created.Title {
		t.Fatalf("Title mismatch: %q vs %q", fetched.Title, created.Title)
	}

	// Get non-existent.
	_, err = store.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent meeting")
	}
}

func TestMeetingStore_CreateDefaultMaxRounds(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMeetingStore(dir)
	if err != nil {
		t.Fatalf("NewMeetingStore: %v", err)
	}

	m, err := store.Create(MeetingRequest{
		Type:         MeetingPlanning,
		Title:        "Planning",
		ProjectID:    "proj-1",
		ChairID:      "worker-a",
		Participants: []string{"worker-a"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if m.MaxRounds != 3 {
		t.Fatalf("expected default MaxRounds 3, got %d", m.MaxRounds)
	}
}

func TestMeetingStore_List(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMeetingStore(dir)
	if err != nil {
		t.Fatalf("NewMeetingStore: %v", err)
	}

	for i := 0; i < 3; i++ {
		_, err := store.Create(MeetingRequest{
			Type:         MeetingReview,
			Title:        "Meeting",
			ProjectID:    "proj-1",
			ChairID:      "worker-a",
			Participants: []string{"worker-a"},
		})
		if err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
		// Small sleep so CreatedAt timestamps differ.
		time.Sleep(time.Millisecond)
	}

	all := store.List()
	if len(all) != 3 {
		t.Fatalf("expected 3 meetings, got %d", len(all))
	}

	// Verify sorted by CreatedAt desc.
	for i := 0; i < len(all)-1; i++ {
		if all[i].CreatedAt.Before(all[i+1].CreatedAt) {
			t.Fatalf("meetings not sorted desc at index %d", i)
		}
	}
}

func TestMeetingStore_ListByProject(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMeetingStore(dir)
	if err != nil {
		t.Fatalf("NewMeetingStore: %v", err)
	}

	store.Create(MeetingRequest{Type: MeetingReview, Title: "A", ProjectID: "proj-1", ChairID: "w", Participants: []string{"w"}})
	store.Create(MeetingRequest{Type: MeetingReview, Title: "B", ProjectID: "proj-2", ChairID: "w", Participants: []string{"w"}})
	store.Create(MeetingRequest{Type: MeetingReview, Title: "C", ProjectID: "proj-1", ChairID: "w", Participants: []string{"w"}})

	result := store.ListByProject("proj-1")
	if len(result) != 2 {
		t.Fatalf("expected 2 meetings for proj-1, got %d", len(result))
	}
	for _, m := range result {
		if m.ProjectID != "proj-1" {
			t.Fatalf("unexpected project %q", m.ProjectID)
		}
	}
}

func TestMeetingStore_ListByStatus(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMeetingStore(dir)
	if err != nil {
		t.Fatalf("NewMeetingStore: %v", err)
	}

	m1, _ := store.Create(MeetingRequest{Type: MeetingReview, Title: "A", ProjectID: "p", ChairID: "w", Participants: []string{"w"}})
	store.Create(MeetingRequest{Type: MeetingReview, Title: "B", ProjectID: "p", ChairID: "w", Participants: []string{"w"}})

	// Update one to in_progress.
	m1.Status = MeetingInProgress
	store.Update(m1)

	scheduled := store.ListByStatus(MeetingScheduled)
	if len(scheduled) != 1 {
		t.Fatalf("expected 1 scheduled, got %d", len(scheduled))
	}
	if scheduled[0].Title != "B" {
		t.Fatalf("expected title B, got %q", scheduled[0].Title)
	}

	inProgress := store.ListByStatus(MeetingInProgress)
	if len(inProgress) != 1 {
		t.Fatalf("expected 1 in_progress, got %d", len(inProgress))
	}
}

func TestMeetingStore_Update(t *testing.T) {
	dir := t.TempDir()
	store, err := NewMeetingStore(dir)
	if err != nil {
		t.Fatalf("NewMeetingStore: %v", err)
	}

	m, _ := store.Create(MeetingRequest{
		Type:         MeetingReview,
		Title:        "Review",
		ProjectID:    "p",
		ChairID:      "w",
		Participants: []string{"w"},
	})

	m.Status = MeetingCompleted
	m.Verdict = "approved"
	if err := store.Update(m); err != nil {
		t.Fatalf("Update: %v", err)
	}

	fetched, _ := store.Get(m.ID)
	if fetched.Status != MeetingCompleted {
		t.Fatalf("expected status %q, got %q", MeetingCompleted, fetched.Status)
	}
	if fetched.Verdict != "approved" {
		t.Fatalf("expected verdict %q, got %q", "approved", fetched.Verdict)
	}

	// Update non-existent.
	err = store.Update(&Meeting{ID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent meeting update")
	}
}

func TestMeetingStore_SaveAndReload(t *testing.T) {
	dir := t.TempDir()
	store1, err := NewMeetingStore(dir)
	if err != nil {
		t.Fatalf("NewMeetingStore: %v", err)
	}

	m1, _ := store1.Create(MeetingRequest{
		Type:         MeetingReview,
		Title:        "Persist Me",
		ProjectID:    "proj-1",
		ChairID:      "worker-a",
		Participants: []string{"worker-a", "worker-b"},
		MaxRounds:    4,
	})

	m2, _ := store1.Create(MeetingRequest{
		Type:         MeetingPlanning,
		Title:        "Plan Meeting",
		ProjectID:    "proj-2",
		ChairID:      "worker-c",
		Participants: []string{"worker-c"},
	})

	// Update m2 status.
	m2.Status = MeetingInProgress
	store1.Update(m2)

	// Create a new store from the same directory — should load persisted data.
	store2, err := NewMeetingStore(dir)
	if err != nil {
		t.Fatalf("NewMeetingStore reload: %v", err)
	}

	all := store2.List()
	if len(all) != 2 {
		t.Fatalf("expected 2 meetings after reload, got %d", len(all))
	}

	reloaded, err := store2.Get(m1.ID)
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}
	if reloaded.Title != "Persist Me" {
		t.Fatalf("title mismatch after reload: %q", reloaded.Title)
	}
	if reloaded.MaxRounds != 4 {
		t.Fatalf("MaxRounds mismatch: %d", reloaded.MaxRounds)
	}

	reloaded2, _ := store2.Get(m2.ID)
	if reloaded2.Status != MeetingInProgress {
		t.Fatalf("status mismatch after reload: %q", reloaded2.Status)
	}
}

// ---------------------------------------------------------------------------
// MeetingEngine tests
// ---------------------------------------------------------------------------

func TestMeetingEngine_Schedule(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewMeetingStore(dir)
	mailbox, _ := NewMailbox(dir)
	wc := &mockWorkerChecker{statuses: map[string]string{}}

	engine := NewMeetingEngine(nil, mailbox, nil, "zh-TW", store, wc)

	m, err := engine.Schedule(MeetingRequest{
		Type:         MeetingReview,
		Title:        "Schedule Test",
		ProjectID:    "proj-1",
		ChairID:      "chair",
		Participants: []string{"worker-a", "worker-b"},
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	if m.Status != MeetingScheduled {
		t.Fatalf("expected status %q, got %q", MeetingScheduled, m.Status)
	}
	if m.Title != "Schedule Test" {
		t.Fatalf("expected title %q, got %q", "Schedule Test", m.Title)
	}

	// Check mailbox notifications.
	msgsA := mailbox.Peek("worker-a")
	if len(msgsA) != 1 {
		t.Fatalf("expected 1 notification for worker-a, got %d", len(msgsA))
	}
	msgsB := mailbox.Peek("worker-b")
	if len(msgsB) != 1 {
		t.Fatalf("expected 1 notification for worker-b, got %d", len(msgsB))
	}
}

func TestMeetingEngine_Schedule_NoMailbox(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewMeetingStore(dir)
	wc := &mockWorkerChecker{statuses: map[string]string{}}

	engine := NewMeetingEngine(nil, nil, nil, "zh-TW", store, wc)

	m, err := engine.Schedule(MeetingRequest{
		Type:         MeetingReview,
		Title:        "No Mailbox Test",
		ProjectID:    "proj-1",
		ChairID:      "chair",
		Participants: []string{"worker-a"},
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	if m.Status != MeetingScheduled {
		t.Fatalf("expected scheduled, got %q", m.Status)
	}
}

func TestMeetingEngine_Start_AllAvailable(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewMeetingStore(dir)
	mailbox, _ := NewMailbox(dir)
	wc := &mockWorkerChecker{
		statuses: map[string]string{
			"worker-a": "idle",
			"worker-b": "idle",
		},
	}

	engine := NewMeetingEngine(nil, mailbox, nil, "zh-TW", store, wc)

	m, _ := engine.Schedule(MeetingRequest{
		Type:         MeetingReview,
		Title:        "Start Test",
		ProjectID:    "proj-1",
		ChairID:      "chair",
		Participants: []string{"worker-a", "worker-b"},
	})

	// Drain schedule notifications.
	mailbox.Deliver("worker-a")
	mailbox.Deliver("worker-b")

	err := engine.Start(context.Background(), m.ID)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	updated, _ := store.Get(m.ID)
	if updated.Status != MeetingInProgress {
		t.Fatalf("expected status %q, got %q", MeetingInProgress, updated.Status)
	}

	// Check start notifications.
	msgsA := mailbox.Peek("worker-a")
	if len(msgsA) != 1 {
		t.Fatalf("expected 1 start notification for worker-a, got %d", len(msgsA))
	}
}

func TestMeetingEngine_Start_SomeBusy(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewMeetingStore(dir)
	wc := &mockWorkerChecker{
		statuses: map[string]string{
			"worker-a": "idle",
			"worker-b": "working",
			"worker-c": "idle",
		},
	}

	engine := NewMeetingEngine(nil, nil, nil, "zh-TW", store, wc)

	m, _ := engine.Schedule(MeetingRequest{
		Type:         MeetingReview,
		Title:        "Busy Test",
		ProjectID:    "proj-1",
		ChairID:      "chair",
		Participants: []string{"worker-a", "worker-b", "worker-c"},
	})

	err := engine.Start(context.Background(), m.ID)
	if err == nil {
		t.Fatal("expected error when participant is busy")
	}
	if !contains(err.Error(), "worker-b") {
		t.Fatalf("error should mention busy worker, got: %v", err)
	}

	// Meeting status should remain scheduled.
	updated, _ := store.Get(m.ID)
	if updated.Status != MeetingScheduled {
		t.Fatalf("expected status %q, got %q", MeetingScheduled, updated.Status)
	}
}

func TestMeetingEngine_Start_WorkerNotFound(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewMeetingStore(dir)
	wc := &mockWorkerChecker{
		statuses: map[string]string{
			"worker-a": "idle",
			// worker-b not in map at all
		},
	}

	engine := NewMeetingEngine(nil, nil, nil, "zh-TW", store, wc)

	m, _ := engine.Schedule(MeetingRequest{
		Type:         MeetingReview,
		Title:        "Missing Worker Test",
		ProjectID:    "proj-1",
		ChairID:      "chair",
		Participants: []string{"worker-a", "worker-b"},
	})

	err := engine.Start(context.Background(), m.ID)
	if err == nil {
		t.Fatal("expected error when participant not found")
	}
	if !contains(err.Error(), "worker-b") {
		t.Fatalf("error should mention missing worker, got: %v", err)
	}
}

func TestMeetingEngine_Cancel(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewMeetingStore(dir)
	mailbox, _ := NewMailbox(dir)
	wc := &mockWorkerChecker{statuses: map[string]string{}}

	engine := NewMeetingEngine(nil, mailbox, nil, "zh-TW", store, wc)

	m, _ := engine.Schedule(MeetingRequest{
		Type:         MeetingReview,
		Title:        "Cancel Test",
		ProjectID:    "proj-1",
		ChairID:      "chair",
		Participants: []string{"worker-a", "worker-b"},
	})

	// Drain schedule notifications.
	mailbox.Deliver("worker-a")
	mailbox.Deliver("worker-b")

	err := engine.Cancel(m.ID)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	updated, _ := store.Get(m.ID)
	if updated.Status != MeetingCancelled {
		t.Fatalf("expected status %q, got %q", MeetingCancelled, updated.Status)
	}

	// Check cancel notifications.
	msgsA := mailbox.Peek("worker-a")
	if len(msgsA) != 1 {
		t.Fatalf("expected 1 cancel notification for worker-a, got %d", len(msgsA))
	}
}

// ---------------------------------------------------------------------------
// checkConsensus tests
// ---------------------------------------------------------------------------

func TestCheckConsensus_Unanimous(t *testing.T) {
	speeches := []Speech{
		{WorkerID: "a", Vote: "approve"},
		{WorkerID: "b", Vote: "approve"},
		{WorkerID: "c", Vote: "approve"},
	}

	reached, verdict := checkConsensus(speeches, 0.67)
	if !reached {
		t.Fatal("expected consensus reached")
	}
	if verdict != "approve" {
		t.Fatalf("expected verdict %q, got %q", "approve", verdict)
	}
}

func TestCheckConsensus_TwoThirds(t *testing.T) {
	speeches := []Speech{
		{WorkerID: "a", Vote: "approve"},
		{WorkerID: "b", Vote: "approve"},
		{WorkerID: "c", Vote: "reject"},
	}

	// 2/3 = 0.666... which meets 0.67 threshold? No, 0.666 < 0.67.
	reached, _ := checkConsensus(speeches, 0.67)
	if reached {
		t.Fatal("2/3 should not meet 0.67 threshold")
	}

	// But at 0.66 threshold it should pass.
	reached, verdict := checkConsensus(speeches, 0.66)
	if !reached {
		t.Fatal("expected consensus at 0.66 threshold")
	}
	if verdict != "approve" {
		t.Fatalf("expected %q, got %q", "approve", verdict)
	}
}

func TestCheckConsensus_NoConsensus(t *testing.T) {
	speeches := []Speech{
		{WorkerID: "a", Vote: "approve"},
		{WorkerID: "b", Vote: "reject"},
		{WorkerID: "c", Vote: "reject"},
		{WorkerID: "d", Vote: "approve"},
	}

	reached, _ := checkConsensus(speeches, 0.67)
	if reached {
		t.Fatal("expected no consensus on split vote")
	}
}

func TestCheckConsensus_Empty(t *testing.T) {
	reached, verdict := checkConsensus(nil, 0.67)
	if reached {
		t.Fatal("expected no consensus on empty speeches")
	}
	if verdict != "" {
		t.Fatalf("expected empty verdict, got %q", verdict)
	}

	reached, verdict = checkConsensus([]Speech{}, 0.67)
	if reached {
		t.Fatal("expected no consensus on empty slice")
	}
	if verdict != "" {
		t.Fatalf("expected empty verdict, got %q", verdict)
	}
}

func TestCheckConsensus_WithAbstain(t *testing.T) {
	speeches := []Speech{
		{WorkerID: "a", Vote: "approve"},
		{WorkerID: "b", Vote: "approve"},
		{WorkerID: "c", Vote: "abstain"},
		{WorkerID: "d", Vote: "abstain"},
	}

	// Denominator should be 2 (non-abstain), 2/2 = 1.0 >= 0.67.
	reached, verdict := checkConsensus(speeches, 0.67)
	if !reached {
		t.Fatal("expected consensus when abstains excluded")
	}
	if verdict != "approve" {
		t.Fatalf("expected %q, got %q", "approve", verdict)
	}
}

func TestCheckConsensus_AllAbstain(t *testing.T) {
	speeches := []Speech{
		{WorkerID: "a", Vote: "abstain"},
		{WorkerID: "b", Vote: "abstain"},
	}

	reached, _ := checkConsensus(speeches, 0.67)
	if reached {
		t.Fatal("expected no consensus when all abstain")
	}
}

func TestCheckConsensus_NoVotes(t *testing.T) {
	speeches := []Speech{
		{WorkerID: "a", Vote: ""},
		{WorkerID: "b", Vote: ""},
	}

	reached, _ := checkConsensus(speeches, 0.67)
	if reached {
		t.Fatal("expected no consensus when no votes cast")
	}
}

// contains is a helper to check substring presence.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstr(s, substr)
}

func searchSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
