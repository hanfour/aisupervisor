package company

import (
	"context"
	"testing"
)

// TestPlanningMeeting_ProducesTaskProposals verifies that a planning meeting
// produces task proposals extracted from the final round's speeches.
func TestPlanningMeeting_ProducesTaskProposals(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewMeetingStore(dir)

	jsonProposals := `Here are my proposed tasks:
` + "```json" + `
[{"title":"Set up CI","description":"Configure GitHub Actions","priority":1},{"title":"Write tests","description":"Add unit tests","priority":2}]
` + "```" + `
VOTE:approve`

	mock := &meetingMockChat{
		responses: []string{jsonProposals},
	}

	engine := NewMeetingEngine(mock, nil, nil, "en", store, &mockWorkerChecker{statuses: map[string]string{}})

	m, _ := store.Create(MeetingRequest{
		Type:         MeetingPlanning,
		Title:        "Sprint Planning",
		ProjectID:    "proj-1",
		ChairID:      "worker-a",
		Participants: []string{"worker-a", "worker-b"},
	})
	m.Status = MeetingInProgress
	store.Update(m)

	proposals, err := engine.RunPlanningMeeting(context.Background(), m, PlanningMeetingConfig{
		Goals: []string{"deliver MVP"},
	})
	if err != nil {
		t.Fatalf("RunPlanningMeeting: %v", err)
	}

	if m.Status != MeetingCompleted {
		t.Fatalf("expected status %q, got %q", MeetingCompleted, m.Status)
	}
	if len(m.Rounds) != 3 {
		t.Fatalf("expected 3 rounds, got %d", len(m.Rounds))
	}
	if len(proposals) == 0 {
		t.Fatal("expected at least 1 task proposal, got 0")
	}

	// Verify proposal content (from final round).
	foundCI := false
	foundTests := false
	for _, p := range proposals {
		if p.Title == "Set up CI" {
			foundCI = true
		}
		if p.Title == "Write tests" {
			foundTests = true
		}
	}
	if !foundCI {
		t.Fatal("expected proposal 'Set up CI' not found")
	}
	if !foundTests {
		t.Fatal("expected proposal 'Write tests' not found")
	}
}

// TestExtractTaskProposals verifies JSON extraction from speech content.
func TestExtractTaskProposals(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{
			name: "json code block",
			content: "Here are tasks:\n```json\n" +
				`[{"title":"Task A","description":"Do A"},{"title":"Task B","description":"Do B"}]` +
				"\n```\nVOTE:approve",
			want: 2,
		},
		{
			name:    "inline json",
			content: `Some text before [{"title":"Inline Task","description":"inline"}] and after`,
			want:    1,
		},
		{
			name:    "no json",
			content: "Just some plain text without any proposals.\nVOTE:approve",
			want:    0,
		},
		{
			name:    "empty round",
			content: "",
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			round := &MeetingRound{
				Number: 3,
				Speeches: []Speech{
					{WorkerID: "worker-a", Content: tt.content},
				},
			}
			got := extractTaskProposals(round)
			if len(got) != tt.want {
				t.Errorf("extractTaskProposals() returned %d proposals, want %d", len(got), tt.want)
			}
		})
	}
}

// TestExtractTaskProposals_NilRound verifies nil round returns nil.
func TestExtractTaskProposals_NilRound(t *testing.T) {
	got := extractTaskProposals(nil)
	if got != nil {
		t.Fatalf("expected nil for nil round, got %v", got)
	}
}

// TestExtractTaskProposals_Dedup verifies that duplicate titles are removed.
func TestExtractTaskProposals_Dedup(t *testing.T) {
	round := &MeetingRound{
		Number: 3,
		Speeches: []Speech{
			{
				WorkerID: "worker-a",
				Content: "```json\n" +
					`[{"title":"Setup DB","description":"from worker A"},{"title":"Write API","description":"from A"}]` +
					"\n```",
			},
			{
				WorkerID: "worker-b",
				Content: "```json\n" +
					`[{"title":"Setup DB","description":"from worker B"},{"title":"Deploy","description":"from B"}]` +
					"\n```",
			},
		},
	}

	got := extractTaskProposals(round)

	// "Setup DB" appears in both speeches — should be deduplicated.
	if len(got) != 3 {
		t.Fatalf("expected 3 unique proposals (Setup DB, Write API, Deploy), got %d", len(got))
	}

	titles := map[string]bool{}
	for _, p := range got {
		if titles[p.Title] {
			t.Fatalf("duplicate title found: %q", p.Title)
		}
		titles[p.Title] = true
	}

	if !titles["Setup DB"] || !titles["Write API"] || !titles["Deploy"] {
		t.Fatalf("unexpected titles: %v", titles)
	}
}
