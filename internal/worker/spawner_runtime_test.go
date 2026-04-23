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

func (f *fakeRuntime) DetectCompletion(_ context.Context, _ *agent.AgentSession) (bool, error) {
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
	if s.RuntimeRegistry() != nil {
		t.Fatalf("expected nil registry before SetRuntimeRegistry")
	}

	reg := agent.NewRuntimeRegistry()
	s.SetRuntimeRegistry(reg)

	if got := s.RuntimeRegistry(); got != reg {
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
