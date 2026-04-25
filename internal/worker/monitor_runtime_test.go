package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/agent"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

// --- Test doubles -----------------------------------------------------------

// fakeTmuxClient satisfies tmux.TmuxClient with scripted pane content. If the
// contents slice is exhausted, the last entry is returned repeatedly. This
// lets us simulate "content keeps changing" to satisfy changeCount gating.
type fakeTmuxClient struct {
	mu         sync.Mutex
	contents   []string
	idx        int
	sessionOK  bool
	captureErr error
}

func (f *fakeTmuxClient) next() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.captureErr != nil {
		return "", f.captureErr
	}
	if len(f.contents) == 0 {
		return "", nil
	}
	c := f.contents[f.idx]
	if f.idx < len(f.contents)-1 {
		f.idx++
	}
	return c, nil
}

func (f *fakeTmuxClient) ListSessions() ([]tmux.SessionInfo, error) {
	return []tmux.SessionInfo{{Name: "test", Windows: 1}}, nil
}
func (f *fakeTmuxClient) ListPanes(session string) ([]tmux.PaneInfo, error) {
	return []tmux.PaneInfo{{SessionName: session, WindowIndex: 0, PaneIndex: 0, Active: true}}, nil
}
func (f *fakeTmuxClient) CapturePane(session string, window, pane, lines int) (string, error) {
	return f.next()
}
func (f *fakeTmuxClient) SendKeys(session string, window, pane int, keys string) error {
	return nil
}
func (f *fakeTmuxClient) SendLiteralKeys(session string, window, pane int, text string) error {
	return nil
}
func (f *fakeTmuxClient) CreateSession(name string) error      { return nil }
func (f *fakeTmuxClient) KillSession(name string) error        { return nil }
func (f *fakeTmuxClient) HasSession(name string) (bool, error) { return f.sessionOK, nil }

// stubRuntime is a minimal agent.AgentRuntime where DetectCompletion is
// script-driven. All other methods return harmless zero values.
type stubRuntime struct {
	name       string
	done       bool
	detectErr  error
	detectCall int
}

func (f *stubRuntime) Name() string { return f.name }
func (f *stubRuntime) Spawn(ctx context.Context, cfg agent.SpawnConfig) (*agent.AgentSession, error) {
	return &agent.AgentSession{RuntimeName: f.name}, nil
}
func (f *stubRuntime) SendPrompt(session *agent.AgentSession, prompt string) error { return nil }
func (f *stubRuntime) CaptureOutput(session *agent.AgentSession, lines int) (string, error) {
	return "", nil
}
func (f *stubRuntime) DetectReady(ctx context.Context, session *agent.AgentSession, timeout time.Duration) error {
	return nil
}
func (f *stubRuntime) DetectCompletion(ctx context.Context, session *agent.AgentSession, content string) (bool, error) {
	f.detectCall++
	if f.detectErr != nil {
		return false, f.detectErr
	}
	return f.done, nil
}
func (f *stubRuntime) ParseTokenUsage(output string) (agent.TokenUsage, error) {
	return agent.TokenUsage{}, nil
}
func (f *stubRuntime) Cleanup(session *agent.AgentSession) error { return nil }
func (f *stubRuntime) MonitoredSessionType() string             { return "stub" }

// withShortGrace temporarily sets the package gracePeriod to d so tests do
// not have to wait 30s for idle checks to become eligible. The returned
// cleanup function restores the original value and MUST be deferred.
func withShortGrace(t *testing.T, d time.Duration) func() {
	t.Helper()
	orig := gracePeriod
	gracePeriod = d
	return func() { gracePeriod = orig }
}

// --- SetRuntimeRegistry -----------------------------------------------------

func TestCompletionMonitor_SetRuntimeRegistry(t *testing.T) {
	m := NewCompletionMonitor(&fakeTmuxClient{})
	if m.runtimeRegistry != nil {
		t.Fatalf("expected nil runtimeRegistry before SetRuntimeRegistry, got %v", m.runtimeRegistry)
	}
	reg := agent.NewRuntimeRegistry()
	m.SetRuntimeRegistry(reg)
	if m.runtimeRegistry != reg {
		t.Fatalf("SetRuntimeRegistry did not store the provided registry")
	}
}

// --- runtimeIdle helper -----------------------------------------------------

func TestCompletionMonitor_RuntimeIdle_NoRegistry(t *testing.T) {
	m := NewCompletionMonitor(&fakeTmuxClient{})
	done, err := m.runtimeIdle(context.Background(), &Worker{CLITool: "claudecode"}, "pane content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Fatalf("expected runtimeIdle=false when no registry is set, got true")
	}
}

func TestCompletionMonitor_RuntimeIdle_NoMatchingRuntime(t *testing.T) {
	m := NewCompletionMonitor(&fakeTmuxClient{})
	reg := agent.NewRuntimeRegistry()
	reg.Register(&stubRuntime{name: "claudecode", done: true})
	m.SetRuntimeRegistry(reg)

	// Worker uses a CLITool no runtime matches.
	done, err := m.runtimeIdle(context.Background(), &Worker{CLITool: "unknown"}, "pane content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Fatalf("expected runtimeIdle=false when no runtime matches CLITool, got true")
	}
}

func TestCompletionMonitor_RuntimeIdle_DelegatesToRuntime(t *testing.T) {
	m := NewCompletionMonitor(&fakeTmuxClient{})
	reg := agent.NewRuntimeRegistry()
	rt := &stubRuntime{name: "claudecode", done: true}
	reg.Register(rt)
	m.SetRuntimeRegistry(reg)

	done, err := m.runtimeIdle(context.Background(), &Worker{CLITool: "claudecode"}, "pane content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Fatalf("expected runtimeIdle=true when runtime reports idle, got false")
	}
	if rt.detectCall != 1 {
		t.Fatalf("expected DetectCompletion to be called once, got %d", rt.detectCall)
	}
}

func TestCompletionMonitor_RuntimeIdle_PropagatesError(t *testing.T) {
	m := NewCompletionMonitor(&fakeTmuxClient{})
	reg := agent.NewRuntimeRegistry()
	wantErr := errors.New("boom")
	reg.Register(&stubRuntime{name: "claudecode", detectErr: wantErr})
	m.SetRuntimeRegistry(reg)

	done, err := m.runtimeIdle(context.Background(), &Worker{CLITool: "claudecode"}, "pane content")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
	if done {
		t.Fatalf("expected runtimeIdle=false on error, got true")
	}
}

// --- WatchForCompletion end-to-end ------------------------------------------

// varyingContents returns a slice of pane captures that look like the worker
// is making progress (3+ distinct strings), culminating in a final idle
// prompt line. This satisfies changeCount >= 3 on subsequent ticks.
func varyingContents(finalLine string) []string {
	return []string{
		"starting up\n",
		"working on step 1...\n",
		"step 2 complete\n",
		"finishing up\n" + finalLine,
	}
}

func TestCompletionMonitor_NoRegistryFallsBackToLegacy(t *testing.T) {
	defer withShortGrace(t, 10*time.Millisecond)()

	tmuxC := &fakeTmuxClient{
		contents:  varyingContents("❯"),
		sessionOK: true,
	}
	m := NewCompletionMonitor(tmuxC)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	w := &Worker{
		TmuxSession:  "s",
		Window:       0,
		Pane:         0,
		CLITool:      "claudecode",
		ModelVersion: "sonnet",
	}
	res, err := m.WatchForCompletion(ctx, w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Reason != "idle_prompt" {
		t.Fatalf("expected Reason=idle_prompt (legacy), got %q", res.Reason)
	}
	if !res.Success {
		t.Fatalf("expected Success=true, got false")
	}
}

func TestCompletionMonitor_RuntimeDetectionPreferred(t *testing.T) {
	defer withShortGrace(t, 10*time.Millisecond)()

	// Pane content is varied to satisfy changeCount gating, but does NOT end
	// in an idle prompt so the legacy isClaudeIdle check would return false.
	// The only way the monitor can return success here is via runtime_idle.
	tmuxC := &fakeTmuxClient{
		contents: []string{
			"starting\n",
			"working...\n",
			"halfway\n",
			"almost done\n",
		},
		sessionOK: true,
	}
	m := NewCompletionMonitor(tmuxC)

	reg := agent.NewRuntimeRegistry()
	reg.Register(&stubRuntime{name: "claudecode", done: true})
	m.SetRuntimeRegistry(reg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	w := &Worker{
		TmuxSession:  "s",
		Window:       0,
		Pane:         0,
		CLITool:      "claudecode",
		ModelVersion: "sonnet",
	}
	res, err := m.WatchForCompletion(ctx, w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Reason != "runtime_idle" {
		t.Fatalf("expected Reason=runtime_idle, got %q", res.Reason)
	}
	if !res.Success {
		t.Fatalf("expected Success=true, got false")
	}
}

func TestCompletionMonitor_RuntimeErrorFallsBackToLegacy(t *testing.T) {
	defer withShortGrace(t, 10*time.Millisecond)()

	// Pane ends in an idle ❯ so legacy detection can succeed.
	tmuxC := &fakeTmuxClient{
		contents:  varyingContents("❯"),
		sessionOK: true,
	}
	m := NewCompletionMonitor(tmuxC)

	reg := agent.NewRuntimeRegistry()
	// Runtime errors on every call — monitor must fall through to legacy.
	reg.Register(&stubRuntime{name: "claudecode", detectErr: errors.New("runtime offline")})
	m.SetRuntimeRegistry(reg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	w := &Worker{
		TmuxSession:  "s",
		Window:       0,
		Pane:         0,
		CLITool:      "claudecode",
		ModelVersion: "sonnet",
	}
	res, err := m.WatchForCompletion(ctx, w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Reason != "idle_prompt" {
		t.Fatalf("expected legacy Reason=idle_prompt after runtime error, got %q", res.Reason)
	}
	if !res.Success {
		t.Fatalf("expected Success=true, got false")
	}
}

// TestCompletionMonitor_NoChangeRequiresIdleShape verifies the gate
// added in PR #24: the no_change exit must NOT fire when the pane is
// stable for noChangeThreshold seconds but is NOT in an idle shape
// (e.g. claude is mid-generation, pane shows progress text and stays
// still during a long LLM roundtrip).
//
// Fixture: 3 unique frames to satisfy changeCount, then a single
// progress-shape frame repeated forever. Without the idleShape guard,
// no_change would fire after noChangeThreshold polls. With the guard,
// it must NOT fire — only ctx timeout returns.
//
// We use haiku (noChangeThreshold = 60 polls) and a 70 s ctx so the
// counter exceeds the threshold and the gate is exercised.
func TestCompletionMonitor_NoChangeRequiresIdleShape(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: blocks on 70s ctx to exercise noChangeThreshold")
	}
	defer withShortGrace(t, 10*time.Millisecond)()

	tmuxC := &fakeTmuxClient{
		contents: []string{
			"step 1\n",
			"step 2\n",
			"step 3\n",
			// Repeats forever after slice exhausted: progress-shape (no
			// bare prompt anywhere in last 5 lines), NOT an idle prompt.
			"✻ Crunched for 30s\n  status bar\n",
		},
		sessionOK: true,
	}
	m := NewCompletionMonitor(tmuxC)

	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
	defer cancel()

	w := &Worker{
		TmuxSession:  "s",
		Window:       0,
		Pane:         0,
		CLITool:      "claudecode",
		ModelVersion: "haiku",
	}
	_, err := m.WatchForCompletion(ctx, w)
	if err == nil {
		t.Fatal("expected ctx deadline (no_change suppressed by idleShape gate), got success")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

// TestCompletionMonitor_TransientIdleDoesNotFire verifies the
// idleStableMinPolls gate added in PR #22: a pane that flickers in and
// out of the idle layout (e.g. claude renders ❯ briefly between
// SendPrompt and starting to generate, then progress indicators replace
// it) must NOT be treated as a completed turn.
//
// The fixture interleaves "idle-shaped" frames with progress frames so
// noChangeCount can never accumulate to 5 — the test is satisfied by the
// monitor failing to fire idle within the bounded ctx, which we surface
// as ctx.DeadlineExceeded.
func TestCompletionMonitor_TransientIdleDoesNotFire(t *testing.T) {
	defer withShortGrace(t, 10*time.Millisecond)()

	tmuxC := &fakeTmuxClient{
		contents: []string{
			// First three entries satisfy changeCount >= 3.
			"step 1\n",
			"step 2\n",
			"step 3\n",
			// Then the pane bounces between idle-looking and busy-looking
			// frames forever. Each tick the content is different from the
			// previous, so noChangeCount stays at 0 — gating must hold.
			"────────\n❯\n────────\n  status\n",
			"✻ Crunched for 1m\n  status\n",
			"────────\n❯\n────────\n  status\n",
			"✻ Crunched for 2m\n  status\n",
			"────────\n❯\n────────\n  status\n",
			"✻ Crunched for 3m\n  status\n",
			// After the slice is exhausted, fakeTmuxClient repeats the last
			// entry forever — that's a busy frame, so still no idle.
		},
		sessionOK: true,
	}
	m := NewCompletionMonitor(tmuxC)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	w := &Worker{
		TmuxSession:  "s",
		Window:       0,
		Pane:         0,
		CLITool:      "claudecode",
		ModelVersion: "sonnet",
	}
	_, err := m.WatchForCompletion(ctx, w)
	if err == nil {
		t.Fatal("expected ctx deadline (idle suppressed by stable-gate), got success")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

// TestCompletionMonitor_StableIdleEventuallyFires verifies the gate
// doesn't permanently block real idle: once content is stable for
// idleStableMinPolls (~5s), idle_prompt fires.
func TestCompletionMonitor_StableIdleEventuallyFires(t *testing.T) {
	defer withShortGrace(t, 10*time.Millisecond)()

	tmuxC := &fakeTmuxClient{
		// 3 unique frames to satisfy changeCount, then a single idle frame
		// repeated indefinitely. Once we tick onto that frame and stay there
		// for 5 more ticks, idle fires.
		contents: []string{
			"step 1\n",
			"step 2\n",
			"step 3\n",
			"────────\n❯\n────────\n  status bar text\n",
		},
		sessionOK: true,
	}
	m := NewCompletionMonitor(tmuxC)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	res, err := m.WatchForCompletion(ctx, &Worker{
		TmuxSession:  "s",
		Window:       0,
		Pane:         0,
		CLITool:      "claudecode",
		ModelVersion: "sonnet",
	})
	if err != nil {
		t.Fatalf("expected idle to fire eventually, got error: %v", err)
	}
	if res.Reason != "idle_prompt" {
		t.Fatalf("expected Reason=idle_prompt, got %q", res.Reason)
	}
}

// Sanity check that a nil runtime value in the registry is treated the same as
// no registration (the Get-returned-nil branch in runtimeIdle).
func TestCompletionMonitor_RuntimeIdle_NilRuntimeSafe(t *testing.T) {
	m := NewCompletionMonitor(&fakeTmuxClient{})
	reg := agent.NewRuntimeRegistry()
	// The registry doesn't allow a nil runtime via Register (Name() would
	// panic), so we instead verify unknown-name path here which also
	// exercises rt == nil defensively. This overlaps with NoMatchingRuntime
	// but documents intent.
	m.SetRuntimeRegistry(reg)
	done, err := m.runtimeIdle(context.Background(), &Worker{CLITool: "anything"}, "pane content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Fatalf("expected runtimeIdle=false, got true")
	}
}
