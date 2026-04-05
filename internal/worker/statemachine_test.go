package worker

import "testing"

func TestStateMachine_ValidTransitions(t *testing.T) {
	tests := []struct {
		from, to WorkerStatus
		valid    bool
	}{
		{WorkerDormant, WorkerSpawning, true},
		{WorkerSpawning, WorkerTrustCheck, true},
		{WorkerSpawning, WorkerFailed, true},
		{WorkerTrustCheck, WorkerReady, true},
		{WorkerReady, WorkerWorking, true},
		{WorkerWorking, WorkerReviewing, true},
		{WorkerWorking, WorkerBlocked, true},
		{WorkerReviewing, WorkerReady, true},
		{WorkerReviewing, WorkerWorking, true},
		{WorkerFailed, WorkerRecovery, true},
		{WorkerRecovery, WorkerReady, true},
		{WorkerRecovery, WorkerFailed, true},
		// Invalid transitions
		{WorkerDormant, WorkerWorking, false},
		{WorkerReady, WorkerReviewing, false},
		{WorkerWorking, WorkerDormant, false},
	}

	for _, tt := range tests {
		sm := NewStateMachine(tt.from)
		err := sm.Transition(tt.to)
		if tt.valid && err != nil {
			t.Errorf("%s → %s should be valid, got error: %v", tt.from, tt.to, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("%s → %s should be invalid, no error returned", tt.from, tt.to)
		}
	}
}

func TestStateMachine_CurrentState(t *testing.T) {
	sm := NewStateMachine(WorkerReady)
	if sm.Current() != WorkerReady {
		t.Errorf("expected Ready, got %s", sm.Current())
	}
	sm.Transition(WorkerWorking)
	if sm.Current() != WorkerWorking {
		t.Errorf("expected Working, got %s", sm.Current())
	}
}
