package verify

import (
	"context"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.TimeoutSec != 300 {
		t.Errorf("TimeoutSec = %d, want 300", cfg.TimeoutSec)
	}
	if cfg.DockerImage != "golang:1.25" {
		t.Errorf("DockerImage = %q, want golang:1.25", cfg.DockerImage)
	}
}

func TestRunAll_EmptyCommands(t *testing.T) {
	cfg := Config{
		Enabled:    true,
		TimeoutSec: 5,
		LintCmd:    "",
		BuildCmd:   "",
		TestCmd:    "",
	}

	runner := NewRunner(cfg, t.TempDir())
	result := runner.RunAll(context.Background(), true)

	if !result.Passed {
		t.Error("empty commands should pass")
	}
	if len(result.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(result.Steps))
	}
}

func TestRunAll_SuccessfulCommand(t *testing.T) {
	cfg := Config{
		Enabled:    true,
		TimeoutSec: 5,
		BuildCmd:   "echo hello",
	}

	runner := NewRunner(cfg, t.TempDir())
	result := runner.RunAll(context.Background(), true)

	if !result.Passed {
		t.Error("echo should pass")
	}
	if len(result.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(result.Steps))
	}
	if result.Steps[0].Step != StepBuild {
		t.Errorf("step = %s, want build", result.Steps[0].Step)
	}
}

func TestRunAll_FailingCommand(t *testing.T) {
	cfg := Config{
		Enabled:    true,
		TimeoutSec: 5,
		LintCmd:    "false",
		BuildCmd:   "echo should-not-run",
	}

	runner := NewRunner(cfg, t.TempDir())
	result := runner.RunAll(context.Background(), true)

	if result.Passed {
		t.Error("should fail on 'false' command")
	}
	// With stopOnFail, should only have 1 step
	if len(result.Steps) != 1 {
		t.Errorf("expected 1 step with stopOnFail, got %d", len(result.Steps))
	}
}

func TestRunAll_ContinueOnFail(t *testing.T) {
	cfg := Config{
		Enabled:    true,
		TimeoutSec: 5,
		LintCmd:    "false",
		BuildCmd:   "echo hello",
		TestCmd:    "echo test",
	}

	runner := NewRunner(cfg, t.TempDir())
	result := runner.RunAll(context.Background(), false)

	if result.Passed {
		t.Error("should fail overall")
	}
	if len(result.Steps) != 3 {
		t.Errorf("expected 3 steps without stopOnFail, got %d", len(result.Steps))
	}
}

func TestVerificationResult_FirstFailure(t *testing.T) {
	result := &VerificationResult{
		Steps: []StepResult{
			{Step: StepLint, Passed: true},
			{Step: StepBuild, Passed: false, Output: "error"},
			{Step: StepTest, Passed: false, Output: "error2"},
		},
	}

	first := result.FirstFailure()
	if first == nil {
		t.Fatal("expected a failure")
	}
	if first.Step != StepBuild {
		t.Errorf("first failure = %s, want build", first.Step)
	}
}

func TestVerificationResult_AllPassed(t *testing.T) {
	result := &VerificationResult{
		Steps: []StepResult{
			{Step: StepLint, Passed: true},
		},
	}

	if result.FirstFailure() != nil {
		t.Error("no failures expected")
	}
}
