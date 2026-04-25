package worker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/agent"
	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/hanfourmini/aisupervisor/internal/growth"
	"github.com/hanfourmini/aisupervisor/internal/project"
)

// fakeRuntime is a minimal AgentRuntime used to test spawner integration.
// It records the sequence of method calls and values passed through so that
// spawnViaRuntime can be exercised without touching tmux or a real CLI.
type fakeRuntime struct {
	name       string
	calls      []string
	spawnCfg   agent.SpawnConfig
	readyErr   error
	spawnErr   error
	sendErr    error
	promptSent string
	session    *agent.AgentSession
}

func (f *fakeRuntime) Name() string { return f.name }

func (f *fakeRuntime) Spawn(ctx context.Context, cfg agent.SpawnConfig) (*agent.AgentSession, error) {
	f.calls = append(f.calls, "Spawn")
	f.spawnCfg = cfg
	if f.spawnErr != nil {
		return nil, f.spawnErr
	}
	f.session = &agent.AgentSession{
		ID:          "fake-session",
		RuntimeName: f.name,
		TmuxSession: "fake-tmux",
		Window:      0,
		Pane:        0,
		WorkDir:     cfg.WorkDir,
		StartedAt:   time.Now(),
	}
	return f.session, nil
}

func (f *fakeRuntime) SendPrompt(_ *agent.AgentSession, prompt string) error {
	f.calls = append(f.calls, "SendPrompt")
	f.promptSent = prompt
	return f.sendErr
}

func (f *fakeRuntime) CaptureOutput(_ *agent.AgentSession, _ int) (string, error) {
	f.calls = append(f.calls, "CaptureOutput")
	return "", nil
}

func (f *fakeRuntime) DetectReady(_ context.Context, _ *agent.AgentSession, _ time.Duration) error {
	f.calls = append(f.calls, "DetectReady")
	return f.readyErr
}

func (f *fakeRuntime) DetectCompletion(_ context.Context, _ *agent.AgentSession, _ string) (bool, error) {
	f.calls = append(f.calls, "DetectCompletion")
	return false, nil
}

func (f *fakeRuntime) ParseTokenUsage(_ string) (agent.TokenUsage, error) {
	f.calls = append(f.calls, "ParseTokenUsage")
	return agent.TokenUsage{}, nil
}

func (f *fakeRuntime) Cleanup(_ *agent.AgentSession) error {
	f.calls = append(f.calls, "Cleanup")
	return nil
}

func (f *fakeRuntime) MonitoredSessionType() string { return "fake" }

// newTestSpawner builds a Spawner with the minimum non-nil maps required for
// the helpers to run without nil-map panics.
func newTestSpawner() *Spawner {
	return &Spawner{
		tierConfigs:    make(map[WorkerTier]TierSpawnConfig),
		skillProfiles:  make(map[string]config.SkillProfile),
		skillOverrides: make(map[string]config.SkillProfileOverride),
	}
}

func TestSpawner_SetRuntimeRegistry(t *testing.T) {
	s := newTestSpawner()
	if s.runtimeRegistry != nil {
		t.Fatalf("expected nil registry before SetRuntimeRegistry")
	}

	reg := agent.NewRuntimeRegistry()
	s.SetRuntimeRegistry(reg)

	if got := s.runtimeRegistry; got != reg {
		t.Fatalf("SetRuntimeRegistry did not store pointer: got %p, want %p", got, reg)
	}
}

func TestSpawner_ResolveRuntimeName(t *testing.T) {
	// A skill tree where the level-1 default picks "ais-agent" from
	// levelDefaults in growth/config_mapper.go.
	treeL1 := growth.NewSkillTree()

	tierCfgs := map[WorkerTier]TierSpawnConfig{
		TierEngineer: {CLITool: "tier-cli"},
	}

	cases := []struct {
		name        string
		worker      *Worker
		tierConfigs map[WorkerTier]TierSpawnConfig
		want        string
	}{
		{
			name:   "default is claude when nothing set",
			worker: &Worker{ID: "w1"},
			want:   "claude",
		},
		{
			name:   "worker.CLITool wins over default",
			worker: &Worker{ID: "w1", CLITool: "aider"},
			want:   "aider",
		},
		{
			name:   "growth overrides worker.CLITool",
			worker: &Worker{ID: "w1", CLITool: "aider", SkillTree: treeL1},
			want:   "ais-agent", // level-1 default per levelDefaults[1]
		},
		{
			name:        "tier overrides growth and worker",
			worker:      &Worker{ID: "w1", Tier: TierEngineer, CLITool: "aider", SkillTree: treeL1},
			tierConfigs: tierCfgs,
			want:        "tier-cli",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestSpawner()
			if tc.tierConfigs != nil {
				s.tierConfigs = tc.tierConfigs
			}
			got := s.resolveRuntimeName(tc.worker)
			if got != tc.want {
				t.Errorf("resolveRuntimeName = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSpawner_BuildSpawnConfig(t *testing.T) {
	autonomous := config.AutonomousDisallowedTools()
	// sanity: make sure our tests are relying on real behaviour
	if len(autonomous) == 0 {
		t.Fatal("AutonomousDisallowedTools() is empty — test assumptions invalid")
	}

	baseProfile := config.SkillProfile{
		ID:              "coder",
		SystemPrompt:    "base-prompt",
		AllowedTools:    []string{"Read", "Edit"},
		DisallowedTools: []string{"WebFetch"},
		Model:           "sonnet",
		PermissionMode:  "acceptEdits",
		ExtraCLIArgs:    "--foo",
	}

	t.Run("skill profile only", func(t *testing.T) {
		s := newTestSpawner()
		s.skillProfiles["coder"] = baseProfile
		w := &Worker{ID: "w1", SkillProfile: "coder"}

		cfg := s.buildSpawnConfig(w, nil, "/tmp/work", "feat/x")

		if cfg.WorkDir != "/tmp/work" {
			t.Errorf("WorkDir: got %q, want /tmp/work", cfg.WorkDir)
		}
		if cfg.Branch != "feat/x" {
			t.Errorf("Branch: got %q, want feat/x", cfg.Branch)
		}
		if cfg.SystemPrompt != "base-prompt" {
			t.Errorf("SystemPrompt: got %q, want base-prompt", cfg.SystemPrompt)
		}
		if cfg.Model != "sonnet" {
			t.Errorf("Model: got %q, want sonnet", cfg.Model)
		}
		if cfg.PermissionMode != "acceptEdits" {
			t.Errorf("PermissionMode: got %q, want acceptEdits", cfg.PermissionMode)
		}
		if cfg.ExtraCLIArgs != "--foo" {
			t.Errorf("ExtraCLIArgs: got %q, want --foo", cfg.ExtraCLIArgs)
		}
		if len(cfg.AllowedTools) != 2 || cfg.AllowedTools[0] != "Read" || cfg.AllowedTools[1] != "Edit" {
			t.Errorf("AllowedTools: got %v, want [Read Edit]", cfg.AllowedTools)
		}
		// AllowedTools should be a copy, not a reference to the profile slice.
		cfg.AllowedTools[0] = "MUTATED"
		if s.skillProfiles["coder"].AllowedTools[0] != "Read" {
			t.Errorf("AllowedTools must be a copy; original profile mutated")
		}
		// DisallowedTools contains the profile's + the autonomous safety set.
		foundWebFetch := false
		foundAutonomous := false
		for _, tool := range cfg.DisallowedTools {
			if tool == "WebFetch" {
				foundWebFetch = true
			}
			if tool == autonomous[0] {
				foundAutonomous = true
			}
		}
		if !foundWebFetch {
			t.Errorf("DisallowedTools missing profile's WebFetch: %v", cfg.DisallowedTools)
		}
		if !foundAutonomous {
			t.Errorf("DisallowedTools missing autonomous safety (%q): %v", autonomous[0], cfg.DisallowedTools)
		}
	})

	t.Run("growth overrides profile and sets envvars", func(t *testing.T) {
		// Seed the skill tree so EffectiveConfig returns level-1 defaults
		// (CLITool: ais-agent, Provider: ollama, Model: llama3,
		// PermissionMode: plan, AllowedTools: [Read Glob Grep], MaxTokenBudget: 50000,
		// ExtraPrompt: "You are a junior developer...").
		tree := growth.NewSkillTree()

		s := newTestSpawner()
		s.skillProfiles["coder"] = baseProfile
		w := &Worker{ID: "w1", SkillProfile: "coder", SkillTree: tree}

		cfg := s.buildSpawnConfig(w, nil, "/tmp/w", "b")

		// Growth overrides Model
		if cfg.Model != "llama3" {
			t.Errorf("Model: expected growth override llama3, got %q", cfg.Model)
		}
		// Growth overrides PermissionMode
		if cfg.PermissionMode != "plan" {
			t.Errorf("PermissionMode: expected growth override 'plan', got %q", cfg.PermissionMode)
		}
		// Growth overrides AllowedTools (complete replace, not append)
		if len(cfg.AllowedTools) != 3 {
			t.Errorf("AllowedTools: expected 3 tools from growth, got %v", cfg.AllowedTools)
		}
		// SystemPrompt is appended, not replaced
		if !strings.Contains(cfg.SystemPrompt, "base-prompt") {
			t.Errorf("SystemPrompt missing profile text: %q", cfg.SystemPrompt)
		}
		if !strings.Contains(cfg.SystemPrompt, "junior developer") {
			t.Errorf("SystemPrompt missing growth ExtraPrompt: %q", cfg.SystemPrompt)
		}
		// EnvVars carry ais-agent Provider + MaxTokens
		if got := cfg.EnvVars["AIS_PROVIDER"]; got != "ollama" {
			t.Errorf("EnvVars[AIS_PROVIDER] = %q, want ollama", got)
		}
		if got := cfg.EnvVars["AIS_MAX_TOKENS"]; got != "50000" {
			t.Errorf("EnvVars[AIS_MAX_TOKENS] = %q, want 50000", got)
		}
	})

	t.Run("no profile no growth gives empty config with autonomous disallowed", func(t *testing.T) {
		s := newTestSpawner()
		w := &Worker{ID: "w1"}
		cfg := s.buildSpawnConfig(w, nil, "/tmp/x", "")

		if cfg.SystemPrompt != "" {
			t.Errorf("SystemPrompt: want empty, got %q", cfg.SystemPrompt)
		}
		if cfg.Model != "" {
			t.Errorf("Model: want empty, got %q", cfg.Model)
		}
		if len(cfg.AllowedTools) != 0 {
			t.Errorf("AllowedTools: want empty, got %v", cfg.AllowedTools)
		}
		if len(cfg.DisallowedTools) == 0 {
			t.Errorf("DisallowedTools should contain autonomous safety set, got empty")
		}
	})

	t.Run("applies_worker_overrides", func(t *testing.T) {
		// Per-worker skill overrides (Phase-2 retro/growth-loop feature) must
		// flow through buildSpawnConfig when the plugin-runtime path is active.
		// Regression coverage for C3: before the fix, skillOverrides[w.ID]
		// was silently dropped because only buildSkillArgs consulted the map.
		s := newTestSpawner()
		s.skillProfiles["coder"] = baseProfile
		s.skillOverrides["w1"] = config.SkillProfileOverride{
			ExtraPrompt:   "extra-override",
			ModelOverride: "opus",
			AddTools:      []string{"Bash"},
			RemoveTools:   []string{"Edit"},
		}
		w := &Worker{ID: "w1", SkillProfile: "coder"}

		cfg := s.buildSpawnConfig(w, nil, "/tmp/w", "b")

		// ExtraPrompt is appended to the profile's SystemPrompt.
		if !strings.Contains(cfg.SystemPrompt, "base-prompt") {
			t.Errorf("SystemPrompt missing profile text: %q", cfg.SystemPrompt)
		}
		if !strings.Contains(cfg.SystemPrompt, "extra-override") {
			t.Errorf("SystemPrompt missing override ExtraPrompt: %q", cfg.SystemPrompt)
		}
		// ModelOverride replaces the profile's Model.
		if cfg.Model != "opus" {
			t.Errorf("Model: got %q, want opus (override)", cfg.Model)
		}
		// AddTools is appended, RemoveTools filters from AllowedTools.
		// Profile has [Read Edit]; override adds Bash and removes Edit → [Read Bash].
		hasRead, hasEdit, hasBash := false, false, false
		for _, tool := range cfg.AllowedTools {
			switch tool {
			case "Read":
				hasRead = true
			case "Edit":
				hasEdit = true
			case "Bash":
				hasBash = true
			}
		}
		if !hasRead {
			t.Errorf("AllowedTools missing Read: %v", cfg.AllowedTools)
		}
		if hasEdit {
			t.Errorf("AllowedTools should not contain Edit after RemoveTools: %v", cfg.AllowedTools)
		}
		if !hasBash {
			t.Errorf("AllowedTools missing Bash from AddTools: %v", cfg.AllowedTools)
		}
	})
}

// TestSpawner_SpawnViaRuntime_CallOrder verifies the plugin lifecycle contract:
// Spawn → DetectReady → SendPrompt, with correct pieces of context passed through.
func TestSpawner_SpawnViaRuntime_CallOrder(t *testing.T) {
	s := newTestSpawner()

	fr := &fakeRuntime{name: "claude"}
	w := &Worker{ID: "w1", Name: "Alice", Tier: TierEngineer}
	tk := &project.Task{
		ID:         "task-1",
		Title:      "build a thing",
		Prompt:     "please implement X",
		Type:       project.TaskTypeCode,
		BranchName: "feat/x",
	}
	proj := &project.Project{Name: "Demo", RepoPath: "/tmp/does-not-exist-for-test"}

	if err := s.spawnViaRuntime(context.Background(), fr, w, tk, proj, "/tmp/workdir"); err != nil {
		t.Fatalf("spawnViaRuntime returned unexpected error: %v", err)
	}

	wantOrder := []string{"Spawn", "DetectReady", "SendPrompt"}
	if len(fr.calls) != len(wantOrder) {
		t.Fatalf("call count mismatch: got %v, want %v", fr.calls, wantOrder)
	}
	for i, want := range wantOrder {
		if fr.calls[i] != want {
			t.Errorf("call[%d] = %q, want %q (full: %v)", i, fr.calls[i], want, fr.calls)
		}
	}
	if fr.spawnCfg.WorkDir != "/tmp/workdir" {
		t.Errorf("SpawnConfig.WorkDir = %q, want /tmp/workdir", fr.spawnCfg.WorkDir)
	}
	if fr.spawnCfg.Branch != "feat/x" {
		t.Errorf("SpawnConfig.Branch = %q, want feat/x", fr.spawnCfg.Branch)
	}
	if fr.promptSent == "" {
		t.Errorf("SendPrompt received empty prompt")
	}

	// Worker state should be updated with session coordinates.
	if w.TmuxSession != "fake-tmux" {
		t.Errorf("worker TmuxSession = %q, want fake-tmux", w.TmuxSession)
	}
	if w.Status != WorkerWorking {
		t.Errorf("worker Status = %q, want working", w.Status)
	}
	if w.CurrentTaskID != "task-1" {
		t.Errorf("worker CurrentTaskID = %q, want task-1", w.CurrentTaskID)
	}
}

// TestSpawner_SpawnViaRuntime_CleansUpOnReadyFailure ensures we invoke Cleanup
// when DetectReady errors out.
func TestSpawner_SpawnViaRuntime_CleansUpOnReadyFailure(t *testing.T) {
	s := newTestSpawner()
	fr := &fakeRuntime{name: "claude", readyErr: errors.New("boom")}

	w := &Worker{ID: "w1", Name: "Bob"}
	tk := &project.Task{ID: "t", Title: "x", Prompt: "p", Type: project.TaskTypeCode}
	proj := &project.Project{Name: "D", RepoPath: "/tmp/nope"}

	err := s.spawnViaRuntime(context.Background(), fr, w, tk, proj, "/tmp/wd")
	if err == nil {
		t.Fatal("expected error when DetectReady fails")
	}
	if !strings.Contains(err.Error(), "ready") {
		t.Errorf("error should mention ready: %v", err)
	}
	// calls: Spawn, DetectReady, Cleanup
	if got, want := len(fr.calls), 3; got != want {
		t.Fatalf("call count: got %d, want %d (%v)", got, want, fr.calls)
	}
	if fr.calls[2] != "Cleanup" {
		t.Errorf("last call = %q, want Cleanup (full: %v)", fr.calls[2], fr.calls)
	}
}

// TestSpawner_SpawnViaRuntime_CleansUpOnSendPromptFailure ensures we invoke
// Cleanup when SendPrompt errors out (mirror of the DetectReady failure path).
func TestSpawner_SpawnViaRuntime_CleansUpOnSendPromptFailure(t *testing.T) {
	s := newTestSpawner()
	fr := &fakeRuntime{name: "claude", sendErr: errors.New("send-boom")}

	w := &Worker{ID: "w1", Name: "Carol"}
	tk := &project.Task{ID: "t", Title: "x", Prompt: "p", Type: project.TaskTypeCode}
	proj := &project.Project{Name: "D", RepoPath: "/tmp/nope"}

	err := s.spawnViaRuntime(context.Background(), fr, w, tk, proj, "/tmp/wd")
	if err == nil {
		t.Fatal("expected error when SendPrompt fails")
	}
	if !strings.Contains(err.Error(), "send prompt") {
		t.Errorf("error should mention send prompt: %v", err)
	}
	// calls: Spawn, DetectReady, SendPrompt, Cleanup
	wantOrder := []string{"Spawn", "DetectReady", "SendPrompt", "Cleanup"}
	if len(fr.calls) != len(wantOrder) {
		t.Fatalf("call count: got %d, want %d (%v)", len(fr.calls), len(wantOrder), fr.calls)
	}
	for i, want := range wantOrder {
		if fr.calls[i] != want {
			t.Errorf("call[%d] = %q, want %q (full: %v)", i, fr.calls[i], want, fr.calls)
		}
	}
}

// TestSpawner_SpawnForTaskInner_ShortCircuitsToRuntime exercises the
// spawnForTaskInner short-circuit block: with a runtime registered matching
// the worker's CLITool, spawnForTaskInner must dispatch to spawnViaRuntime
// WITHOUT touching the legacy tmux bootstrap below it.
//
// Non-code task + empty BranchName is used so steps 1-2 (git branch setup)
// are skipped, letting the short-circuit run without needing a mock gitOps
// or tmux client.
//
// Guards I6 from the PR #15 review: the conditional at spawner.go:505-509
// was previously tested only by directly calling spawnViaRuntime, never
// through spawnForTaskInner.
func TestSpawner_SpawnForTaskInner_ShortCircuitsToRuntime(t *testing.T) {
	s := newTestSpawner()

	fr := &fakeRuntime{name: "fake-cli"}
	reg := agent.NewRuntimeRegistry()
	reg.Register(fr)
	s.SetRuntimeRegistry(reg)

	w := &Worker{
		ID:      "w-short",
		Name:    "ShortCircuit",
		Tier:    TierEngineer,
		CLITool: "fake-cli",
	}
	// Non-code task with no branch → skips git branch work AND worktree setup.
	tk := &project.Task{
		ID:     "task-ndc",
		Title:  "research thing",
		Prompt: "do the research",
		Type:   project.TaskTypePRD,
	}
	proj := &project.Project{Name: "Demo", RepoPath: "/tmp/nope"}

	if err := s.spawnForTaskInner(context.Background(), w, tk, proj); err != nil {
		t.Fatalf("spawnForTaskInner returned unexpected error: %v", err)
	}

	// fakeRuntime should have received the full spawnViaRuntime sequence.
	wantOrder := []string{"Spawn", "DetectReady", "SendPrompt"}
	if len(fr.calls) < len(wantOrder) {
		t.Fatalf("expected runtime to be driven via short-circuit; got calls=%v", fr.calls)
	}
	for i, want := range wantOrder {
		if fr.calls[i] != want {
			t.Errorf("short-circuit call[%d] = %q, want %q (full: %v)", i, fr.calls[i], want, fr.calls)
		}
	}

	// Worker state should reflect the fake session, not the legacy-path
	// "aiworker-<id>" session name.
	if w.TmuxSession != "fake-tmux" {
		t.Errorf("worker TmuxSession = %q, want fake-tmux (short-circuit did not fire)", w.TmuxSession)
	}
}

// TestSpawner_SpawnForTaskInner_NoMatchingRuntimeKeepsLegacy ensures that
// when a registry is wired but NO runtime matches the worker's CLITool, the
// short-circuit is NOT taken and the legacy path runs. We assert this by
// registering a fakeRuntime under a different name and confirming the
// fakeRuntime was never invoked.
//
// Pins the default-on-no-match semantics: unknown CLITool falls through to
// the existing legacy bootstrap rather than silently crashing or erroring.
func TestSpawner_SpawnForTaskInner_NoMatchingRuntimeKeepsLegacy(t *testing.T) {
	s := newTestSpawner()

	fr := &fakeRuntime{name: "does-not-match"}
	reg := agent.NewRuntimeRegistry()
	reg.Register(fr)
	s.SetRuntimeRegistry(reg)

	w := &Worker{ID: "w-legacy", Name: "Legacy", Tier: TierEngineer, CLITool: "fake-cli"}
	tk := &project.Task{ID: "task-lg", Title: "t", Prompt: "p", Type: project.TaskTypePRD}
	proj := &project.Project{Name: "D", RepoPath: "/tmp/nope"}

	// Legacy path will panic on nil tmuxClient — we catch it and assert we
	// got past the short-circuit.
	defer func() {
		_ = recover() // expected — legacy path touches nil tmuxClient
	}()
	_ = s.spawnForTaskInner(context.Background(), w, tk, proj)

	if len(fr.calls) != 0 {
		t.Errorf("registered-but-unmatched runtime got invoked: %v", fr.calls)
	}
}

// TestSpawner_SpawnViaRuntime_FallsBackToClaudeOnReadyFailure verifies that
// when a non-"claude" runtime (e.g. ais-agent) fails DetectReady — typically
// because the CLI crashed at startup with no banner — the spawner
// automatically retries with the "claude" runtime, avoiding leaving the
// user stuck in a retry loop.
//
// This guards against the real-world case where ais-agent's bundled binary
// rejects the configured provider (e.g. "ollama") and exits immediately.
func TestSpawner_SpawnViaRuntime_FallsBackToClaudeOnReadyFailure(t *testing.T) {
	s := newTestSpawner()

	failing := &fakeRuntime{name: "ais-agent", readyErr: errors.New("timeout")}
	good := &fakeRuntime{name: "claude"}
	reg := agent.NewRuntimeRegistry()
	reg.Register(failing)
	reg.Register(good)
	s.SetRuntimeRegistry(reg)

	w := &Worker{ID: "w-fb", Name: "FB", Tier: TierEngineer, CLITool: "ais-agent"}
	tk := &project.Task{ID: "t-fb", Title: "x", Prompt: "p", Type: project.TaskTypeCode}
	proj := &project.Project{Name: "D", RepoPath: "/tmp/nope"}

	if err := s.spawnViaRuntime(context.Background(), failing, w, tk, proj, "/tmp/wd"); err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}

	// failing runtime: Spawn, DetectReady(fail), Cleanup
	wantFailing := []string{"Spawn", "DetectReady", "Cleanup"}
	if len(failing.calls) != len(wantFailing) {
		t.Fatalf("failing runtime calls = %v, want %v", failing.calls, wantFailing)
	}
	for i, want := range wantFailing {
		if failing.calls[i] != want {
			t.Errorf("failing[%d] = %q, want %q", i, failing.calls[i], want)
		}
	}
	// claude runtime: Spawn, DetectReady, SendPrompt (full happy path)
	wantGood := []string{"Spawn", "DetectReady", "SendPrompt"}
	if len(good.calls) != len(wantGood) {
		t.Fatalf("claude fallback runtime calls = %v, want %v (strict match)", good.calls, wantGood)
	}
	for i, want := range wantGood {
		if good.calls[i] != want {
			t.Errorf("claude[%d] = %q, want %q", i, good.calls[i], want)
		}
	}
	if w.TmuxSession != "fake-tmux" {
		t.Errorf("worker TmuxSession = %q, want fake-tmux from fallback", w.TmuxSession)
	}
}

// TestSpawner_SpawnViaRuntime_NoFallbackWhenAlreadyClaude guards against
// infinite recursion: when "claude" itself fails DetectReady, we must NOT
// recurse back into "claude" again — the error propagates up as before.
func TestSpawner_SpawnViaRuntime_NoFallbackWhenAlreadyClaude(t *testing.T) {
	s := newTestSpawner()

	failing := &fakeRuntime{name: "claude", readyErr: errors.New("timeout")}
	reg := agent.NewRuntimeRegistry()
	reg.Register(failing)
	s.SetRuntimeRegistry(reg)

	w := &Worker{ID: "w-nc", Name: "NC"}
	tk := &project.Task{ID: "t-nc", Title: "x", Prompt: "p", Type: project.TaskTypeCode}
	proj := &project.Project{Name: "D", RepoPath: "/tmp/nope"}

	err := s.spawnViaRuntime(context.Background(), failing, w, tk, proj, "/tmp/wd")
	if err == nil {
		t.Fatal("expected error when claude itself times out (no fallback target)")
	}
	// calls: exactly Spawn, DetectReady, Cleanup — no second Spawn attempt.
	if len(failing.calls) != 3 {
		t.Fatalf("expected exactly 3 calls (no recursion), got %v", failing.calls)
	}
}

// TestSpawner_SpawnViaRuntime_NoFallbackWhenClaudeMissing covers the case
// where the requested runtime fails but the registry has no "claude" to
// fall back to. The original error must propagate unchanged.
func TestSpawner_SpawnViaRuntime_NoFallbackWhenClaudeMissing(t *testing.T) {
	s := newTestSpawner()

	failing := &fakeRuntime{name: "ais-agent", readyErr: errors.New("timeout")}
	reg := agent.NewRuntimeRegistry()
	reg.Register(failing)
	s.SetRuntimeRegistry(reg)

	w := &Worker{ID: "w-nocl", Name: "NoClaude"}
	tk := &project.Task{ID: "t-nocl", Title: "x", Prompt: "p", Type: project.TaskTypeCode}
	proj := &project.Project{Name: "D", RepoPath: "/tmp/nope"}

	err := s.spawnViaRuntime(context.Background(), failing, w, tk, proj, "/tmp/wd")
	if err == nil {
		t.Fatal("expected error when ais-agent fails and claude is not registered")
	}
	if errors.Is(err, ErrRuntimeFallbackExhausted) {
		t.Errorf("should NOT wrap with ErrRuntimeFallbackExhausted — no fallback was attempted")
	}
	// Exactly Spawn, DetectReady, Cleanup — no recursion.
	if len(failing.calls) != 3 {
		t.Fatalf("expected 3 calls on ais-agent (no claude to fall back to), got %v", failing.calls)
	}
}

// TestSpawner_SpawnViaRuntime_NoFallbackWhenContextCancelled verifies we
// respect caller cancellation: if ctx is already cancelled when DetectReady
// returns, we propagate the original ready error without attempting a
// fallback the user didn't ask for.
func TestSpawner_SpawnViaRuntime_NoFallbackWhenContextCancelled(t *testing.T) {
	s := newTestSpawner()

	failing := &fakeRuntime{name: "ais-agent", readyErr: context.Canceled}
	good := &fakeRuntime{name: "claude"}
	reg := agent.NewRuntimeRegistry()
	reg.Register(failing)
	reg.Register(good)
	s.SetRuntimeRegistry(reg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel so ctx.Err() != nil when spawnViaRuntime checks

	w := &Worker{ID: "w-cc", Name: "Cancelled"}
	tk := &project.Task{ID: "t-cc", Title: "x", Prompt: "p", Type: project.TaskTypeCode}
	proj := &project.Project{Name: "D", RepoPath: "/tmp/nope"}

	err := s.spawnViaRuntime(ctx, failing, w, tk, proj, "/tmp/wd")
	if err == nil {
		t.Fatal("expected ready-error to propagate even when ctx is cancelled")
	}
	if errors.Is(err, ErrRuntimeFallbackExhausted) {
		t.Errorf("ctx-cancel path must NOT invoke the fallback machinery, got %v", err)
	}
	// good (claude) must never have been touched.
	if len(good.calls) != 0 {
		t.Errorf("claude runtime was invoked despite ctx cancellation: %v", good.calls)
	}
}

// TestSpawner_SpawnViaRuntime_FallbackExhaustedWhenClaudeAlsoFails verifies
// that when the primary runtime AND the claude fallback both fail, the
// returned error wraps ErrRuntimeFallbackExhausted so ClassifyError routes
// it to ActionAbandon.
func TestSpawner_SpawnViaRuntime_FallbackExhaustedWhenClaudeAlsoFails(t *testing.T) {
	s := newTestSpawner()

	failing := &fakeRuntime{name: "ais-agent", readyErr: errors.New("ais timeout")}
	claudeFailing := &fakeRuntime{name: "claude", readyErr: errors.New("claude timeout")}
	reg := agent.NewRuntimeRegistry()
	reg.Register(failing)
	reg.Register(claudeFailing)
	s.SetRuntimeRegistry(reg)

	w := &Worker{ID: "w-ex", Name: "Exhausted"}
	tk := &project.Task{ID: "t-ex", Title: "x", Prompt: "p", Type: project.TaskTypeCode}
	proj := &project.Project{Name: "D", RepoPath: "/tmp/nope"}

	err := s.spawnViaRuntime(context.Background(), failing, w, tk, proj, "/tmp/wd")
	if err == nil {
		t.Fatal("expected error when both runtimes fail")
	}
	if !errors.Is(err, ErrRuntimeFallbackExhausted) {
		t.Errorf("error must wrap ErrRuntimeFallbackExhausted, got %v", err)
	}
	if ClassifyError(err) != ActionAbandon {
		t.Errorf("ClassifyError should return ActionAbandon on exhausted fallback, got %v", ClassifyError(err))
	}

	// ais-agent: Spawn, DetectReady, Cleanup (3 calls)
	if len(failing.calls) != 3 {
		t.Errorf("ais-agent calls = %v, want 3", failing.calls)
	}
	// claude: Spawn, DetectReady, Cleanup (3 calls — same failure path)
	if len(claudeFailing.calls) != 3 {
		t.Errorf("claude fallback calls = %v, want 3", claudeFailing.calls)
	}
}

// TestSpawner_SpawnViaRuntime_FallbackSendPromptFailure ensures a fallback
// that gets past ready but fails on SendPrompt also surfaces as an
// ErrRuntimeFallbackExhausted-wrapped error (so SpawnForTask abandons
// rather than retrying the same failure).
func TestSpawner_SpawnViaRuntime_FallbackSendPromptFailure(t *testing.T) {
	s := newTestSpawner()

	failing := &fakeRuntime{name: "ais-agent", readyErr: errors.New("ais timeout")}
	claudeSendFail := &fakeRuntime{name: "claude", sendErr: errors.New("send boom")}
	reg := agent.NewRuntimeRegistry()
	reg.Register(failing)
	reg.Register(claudeSendFail)
	s.SetRuntimeRegistry(reg)

	w := &Worker{ID: "w-sf", Name: "SendFail"}
	tk := &project.Task{ID: "t-sf", Title: "x", Prompt: "p", Type: project.TaskTypeCode}
	proj := &project.Project{Name: "D", RepoPath: "/tmp/nope"}

	err := s.spawnViaRuntime(context.Background(), failing, w, tk, proj, "/tmp/wd")
	if err == nil {
		t.Fatal("expected error when fallback SendPrompt fails")
	}
	if !errors.Is(err, ErrRuntimeFallbackExhausted) {
		t.Errorf("error must wrap ErrRuntimeFallbackExhausted, got %v", err)
	}
	// claude got past DetectReady, attempted SendPrompt, then Cleanup.
	wantClaude := []string{"Spawn", "DetectReady", "SendPrompt", "Cleanup"}
	if len(claudeSendFail.calls) != len(wantClaude) {
		t.Fatalf("claude fallback call sequence = %v, want %v", claudeSendFail.calls, wantClaude)
	}
	for i, want := range wantClaude {
		if claudeSendFail.calls[i] != want {
			t.Errorf("claude[%d] = %q, want %q", i, claudeSendFail.calls[i], want)
		}
	}
}

// TestSpawner_SpawnViaRuntime_FallbackSanitizesModel verifies that when
// fallback fires, the SpawnConfig delivered to the claude runtime has its
// Model + ExtraCLIArgs cleared (the two runtime-specific fields), while
// other safety/policy fields (PermissionMode, AllowedTools,
// DisallowedTools, SystemPrompt) pass through to claude unchanged.
//
// Without sanitization, runtime-specific model names like "llama3" (used
// by ais-agent's ollama provider) would be passed verbatim to claude,
// which rejects them and sits at an idle prompt printing "Run /model to
// pick a different model" forever — task never progresses.
func TestSpawner_SpawnViaRuntime_FallbackSanitizesModel(t *testing.T) {
	s := newTestSpawner()
	// Worker requests "ais-agent" — its growth config seeds Model=llama3 via
	// buildSpawnConfig. Simulate by pinning the model + a runtime-specific
	// ExtraCLIArgs and runtime-agnostic policy fields in the SkillProfile.
	s.skillProfiles["junior"] = config.SkillProfile{
		ID:              "junior",
		Model:           "llama3",
		PermissionMode:  "plan",
		AllowedTools:    []string{"Read", "Glob"},
		DisallowedTools: []string{"Write"},
		SystemPrompt:    "you are a junior dev",
		ExtraCLIArgs:    "--max-tokens 50000",
	}

	failing := &fakeRuntime{name: "ais-agent", readyErr: errors.New("timeout")}
	good := &fakeRuntime{name: "claude"}
	reg := agent.NewRuntimeRegistry()
	reg.Register(failing)
	reg.Register(good)
	s.SetRuntimeRegistry(reg)

	w := &Worker{ID: "w-sm", Name: "Sanitize", Tier: TierEngineer, CLITool: "ais-agent", SkillProfile: "junior"}
	tk := &project.Task{ID: "t-sm", Title: "x", Prompt: "p", Type: project.TaskTypeCode}
	proj := &project.Project{Name: "D", RepoPath: "/tmp/nope"}

	if err := s.spawnViaRuntime(context.Background(), failing, w, tk, proj, "/tmp/wd"); err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}

	// ais-agent saw the original cfg in full.
	if failing.spawnCfg.Model != "llama3" {
		t.Errorf("ais-agent SpawnConfig.Model = %q, want %q", failing.spawnCfg.Model, "llama3")
	}
	if failing.spawnCfg.ExtraCLIArgs != "--max-tokens 50000" {
		t.Errorf("ais-agent ExtraCLIArgs = %q, want %q", failing.spawnCfg.ExtraCLIArgs, "--max-tokens 50000")
	}
	// claude (fallback) MUST receive sanitized runtime-specific fields...
	if good.spawnCfg.Model != "" {
		t.Errorf("claude fallback Model = %q, want \"\"", good.spawnCfg.Model)
	}
	if good.spawnCfg.ExtraCLIArgs != "" {
		t.Errorf("claude fallback ExtraCLIArgs = %q, want \"\"", good.spawnCfg.ExtraCLIArgs)
	}
	// ... but inherit the safety/policy fields verbatim.
	if good.spawnCfg.PermissionMode != "plan" {
		t.Errorf("claude fallback PermissionMode = %q, want %q (must pass through)",
			good.spawnCfg.PermissionMode, "plan")
	}
	if got, want := strings.Join(good.spawnCfg.AllowedTools, ","), "Read,Glob"; got != want {
		t.Errorf("claude fallback AllowedTools = %q, want %q (must pass through)", got, want)
	}
	// DisallowedTools also has the autonomous safety net merged in (Skill,
	// EnterPlanMode, ExitPlanMode); just assert the SkillProfile-supplied
	// "Write" is preserved.
	hasWrite := false
	for _, d := range good.spawnCfg.DisallowedTools {
		if d == "Write" {
			hasWrite = true
		}
	}
	if !hasWrite {
		t.Errorf("claude fallback DisallowedTools = %v, want to contain \"Write\" (must pass through)",
			good.spawnCfg.DisallowedTools)
	}
	if good.spawnCfg.SystemPrompt != "you are a junior dev" {
		t.Errorf("claude fallback SystemPrompt = %q, want %q (must pass through)",
			good.spawnCfg.SystemPrompt, "you are a junior dev")
	}
}

// TestSanitizeForClaudeFallback_PureUnit covers the helper directly so a
// future refactor that adds more sanitized fields fails this test rather
// than hiding a regression behind the spawnViaRuntime mock plumbing.
//
// Cleared fields (per docstring): Model, ExtraCLIArgs.
// Pass-through fields: WorkDir, Branch, PermissionMode, AllowedTools,
// DisallowedTools, SystemPrompt, EnvVars.
func TestSanitizeForClaudeFallback_PureUnit(t *testing.T) {
	in := agent.SpawnConfig{
		WorkDir:         "/tmp/wd",
		Branch:          "feat/x",
		Model:           "llama3",
		PermissionMode:  "bypassPermissions",
		AllowedTools:    []string{"Read", "Glob"},
		DisallowedTools: []string{"Edit"},
		SystemPrompt:    "you are helpful",
		ExtraCLIArgs:    "--max-tokens 50000",
		EnvVars:         map[string]string{"AIS_PROVIDER": "ollama"},
	}
	out := sanitizeForClaudeFallback(in)

	// Cleared fields.
	if out.Model != "" {
		t.Errorf("Model = %q, want \"\"", out.Model)
	}
	if out.ExtraCLIArgs != "" {
		t.Errorf("ExtraCLIArgs = %q, want \"\"", out.ExtraCLIArgs)
	}
	// Pass-through scalar fields.
	if out.WorkDir != in.WorkDir || out.Branch != in.Branch ||
		out.PermissionMode != in.PermissionMode ||
		out.SystemPrompt != in.SystemPrompt {
		t.Errorf("scalar fields diverged: in=%+v out=%+v", in, out)
	}
	if len(out.AllowedTools) != len(in.AllowedTools) || len(out.DisallowedTools) != len(in.DisallowedTools) {
		t.Errorf("tool slices changed shape")
	}
	// EnvVars passes through (claude reads no AIS_* keys; harmless).
	if got := out.EnvVars["AIS_PROVIDER"]; got != "ollama" {
		t.Errorf("EnvVars[AIS_PROVIDER] = %q, want %q", got, "ollama")
	}
	// Input must not be mutated (sanitize returns a copy).
	if in.Model != "llama3" {
		t.Errorf("input was mutated: in.Model = %q", in.Model)
	}
	if in.ExtraCLIArgs != "--max-tokens 50000" {
		t.Errorf("input was mutated: in.ExtraCLIArgs = %q", in.ExtraCLIArgs)
	}
}
