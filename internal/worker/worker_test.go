package worker

import "testing"

func TestEffectiveTierDefault(t *testing.T) {
	w := &Worker{}
	if w.EffectiveTier() != TierEngineer {
		t.Fatalf("expected engineer, got %s", w.EffectiveTier())
	}
}

func TestEffectiveTierExplicit(t *testing.T) {
	tests := []struct {
		tier WorkerTier
		want WorkerTier
	}{
		{TierConsultant, TierConsultant},
		{TierManager, TierManager},
		{TierEngineer, TierEngineer},
		{"", TierEngineer},
	}
	for _, tt := range tests {
		w := &Worker{Tier: tt.tier}
		if w.EffectiveTier() != tt.want {
			t.Errorf("Tier=%q: got %s, want %s", tt.tier, w.EffectiveTier(), tt.want)
		}
	}
}

func TestWorkerStatusConstants(t *testing.T) {
	// Ensure status constants are distinct
	statuses := []WorkerStatus{WorkerIdle, WorkerWorking, WorkerWaiting, WorkerFinished, WorkerError}
	seen := make(map[WorkerStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Fatalf("duplicate status: %s", s)
		}
		seen[s] = true
	}
}
