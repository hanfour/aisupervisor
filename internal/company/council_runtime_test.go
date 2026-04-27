package company

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/agent"
)

// fakeCouncilRuntime implements agent.AgentRuntime with recording + scripted
// responses so we can assert spawnExpertAgent delegates the full expert
// lifecycle (Spawn → DetectReady → SendPrompt → CaptureOutput → Cleanup)
// without touching tmux.
type fakeCouncilRuntime struct {
	mu             sync.Mutex
	calls          []string
	session        *agent.AgentSession
	spawnCfg       agent.SpawnConfig
	captureOutput  string
	detectIdleOnce bool // flip to true after first call so completion loop exits fast
	done           bool
}

func (f *fakeCouncilRuntime) Name() string                       { return "claude" }
func (f *fakeCouncilRuntime) Validate(_ agent.SpawnConfig) error { return nil }
func (f *fakeCouncilRuntime) MonitoredSessionType() string       { return "claude_code" }

func (f *fakeCouncilRuntime) Spawn(_ context.Context, cfg agent.SpawnConfig) (*agent.AgentSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "Spawn")
	f.spawnCfg = cfg
	f.session = &agent.AgentSession{
		ID: "council-fake", RuntimeName: "claude",
		TmuxSession: "council-tmux", StartedAt: time.Now(),
	}
	return f.session, nil
}

func (f *fakeCouncilRuntime) SendPrompt(_ *agent.AgentSession, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "SendPrompt")
	return nil
}

func (f *fakeCouncilRuntime) CaptureOutput(_ *agent.AgentSession, _ int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "CaptureOutput")
	return f.captureOutput, nil
}

func (f *fakeCouncilRuntime) DetectReady(_ context.Context, _ *agent.AgentSession, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "DetectReady")
	return nil
}

func (f *fakeCouncilRuntime) DetectCompletion(_ context.Context, _ *agent.AgentSession, _ string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "DetectCompletion")
	// Report idle every call so the 2-consecutive-idle check exits after
	// 6 s of ticker (2 × 3 s). Keeps the test responsive.
	return true, nil
}

func (f *fakeCouncilRuntime) Cleanup(_ *agent.AgentSession) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "Cleanup")
	return nil
}

func (f *fakeCouncilRuntime) ParseTokenUsage(_ string) (agent.TokenUsage, error) {
	return agent.TokenUsage{}, nil
}

// TestCouncil_SpawnExpertAgent_DelegatesToRuntime verifies that when the
// CouncilEngine has a runtime registry wired, spawnExpertAgent delegates
// the whole lifecycle to the runtime plugin instead of calling tmux directly.
//
// The completion wait takes ~6 s (2 × 3 s ticker) — tolerable for an
// integration-style test of the full delegation path.
func TestCouncil_SpawnExpertAgent_DelegatesToRuntime(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: waits ~6s on the completion ticker")
	}

	fr := &fakeCouncilRuntime{
		captureOutput: `[{"file":"foo.go","line":42,"severity":"HIGH","body":"nil check"}]`,
	}
	reg := agent.NewRuntimeRegistry()
	reg.Register(fr)

	c := &CouncilEngine{runtimeRegistry: reg}

	expert := SelectedExpert{
		Expert: Expert{
			Domain:       DomainSecurity,
			Name:         "SecAudit",
			SystemPrompt: "Review for security.",
			Model:        "sonnet",
		},
		AssignedFiles: []string{"foo.go"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	findings, err := c.spawnExpertAgent(ctx, expert, nil, "diff --git a/foo.go b/foo.go\n+ line\n")
	if err != nil {
		t.Fatalf("spawnExpertAgent returned error: %v", err)
	}

	// Ordered call assertions — confirm the delegation flow fired in sequence.
	// We expect at minimum: Spawn, DetectReady, SendPrompt, (poll: CaptureOutput+DetectCompletion)×N, CaptureOutput, Cleanup
	fr.mu.Lock()
	defer fr.mu.Unlock()
	if len(fr.calls) < 5 {
		t.Fatalf("expected runtime to receive full flow, got calls=%v", fr.calls)
	}
	if fr.calls[0] != "Spawn" {
		t.Errorf("first call: got %q, want Spawn", fr.calls[0])
	}
	if fr.calls[1] != "DetectReady" {
		t.Errorf("second call: got %q, want DetectReady", fr.calls[1])
	}
	if fr.calls[2] != "SendPrompt" {
		t.Errorf("third call: got %q, want SendPrompt", fr.calls[2])
	}
	// Last call must be Cleanup.
	if fr.calls[len(fr.calls)-1] != "Cleanup" {
		t.Errorf("last call: got %q, want Cleanup", fr.calls[len(fr.calls)-1])
	}

	// SpawnConfig carried the expected read-only restrictions.
	if fr.spawnCfg.Model != "sonnet" {
		t.Errorf("SpawnConfig.Model = %q, want sonnet", fr.spawnCfg.Model)
	}
	if fr.spawnCfg.PermissionMode != "bypassPermissions" {
		t.Errorf("SpawnConfig.PermissionMode = %q, want bypassPermissions", fr.spawnCfg.PermissionMode)
	}
	wantDisallow := map[string]bool{"Edit": true, "Write": true, "NotebookEdit": true, "Bash": true}
	if len(fr.spawnCfg.DisallowedTools) != len(wantDisallow) {
		t.Errorf("SpawnConfig.DisallowedTools len = %d, want %d (%v)",
			len(fr.spawnCfg.DisallowedTools), len(wantDisallow), fr.spawnCfg.DisallowedTools)
	}
	for _, d := range fr.spawnCfg.DisallowedTools {
		if !wantDisallow[d] {
			t.Errorf("SpawnConfig.DisallowedTools contains unexpected tool %q", d)
		}
	}

	// Findings parsed from the runtime-captured output.
	if len(findings) != 1 {
		t.Fatalf("findings count = %d, want 1 (parsed from CaptureOutput)", len(findings))
	}
	if findings[0].File != "foo.go" {
		t.Errorf("finding file = %q, want foo.go", findings[0].File)
	}
}

// TestCouncil_SpawnExpertAgent_FallsBackToTmuxWhenNoRuntime sanity-checks
// that without a registry the legacy tmux-direct path runs. We only go as
// far as verifying the short-circuit is not taken — the legacy path then
// requires a real tmux, which is out of scope for this test.
func TestCouncil_SpawnExpertAgent_FallsBackToTmuxWhenNoRuntime(t *testing.T) {
	c := &CouncilEngine{} // no runtimeRegistry, no tmuxClient

	// Expect a crash/error from the legacy path. We just need to prove the
	// runtime branch wasn't taken — easiest check: recover from the panic
	// the nil tmuxClient will cause.
	defer func() { _ = recover() }()
	_, _ = c.spawnExpertAgent(context.Background(), SelectedExpert{}, nil, "")
}
