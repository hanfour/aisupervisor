package worker

import "testing"

func TestRecoveryManager_DetectScenario(t *testing.T) {
	rm := NewRecoveryManager()

	tests := []struct {
		paneContent string
		expected    RecoveryScenario
	}{
		{"Do you trust this project? (y/n)", RecoveryTrustPrompt},
		{"fatal: not a git repository", RecoveryStaleBranch},
		{"compilation failed: undefined reference", RecoveryCompileError},
		{"This conversation has tokens remaining: 0", RecoveryTokenExhausted},
		{"everything is fine", RecoveryNone},
	}

	for _, tt := range tests {
		scenario := rm.DetectScenario(tt.paneContent)
		if scenario != tt.expected {
			t.Errorf("pane %q: expected %s, got %s", tt.paneContent[:20], tt.expected, scenario)
		}
	}
}

func TestRecoveryManager_Attempt(t *testing.T) {
	rm := NewRecoveryManager()
	rm.MaxAttempts = 1

	shouldRetry := rm.ShouldRetry("w1", RecoveryTrustPrompt)
	if !shouldRetry {
		t.Error("first attempt should allow retry")
	}

	rm.RecordAttempt("w1", RecoveryTrustPrompt)

	shouldRetry = rm.ShouldRetry("w1", RecoveryTrustPrompt)
	if shouldRetry {
		t.Error("should escalate after max attempts")
	}
}
