# Phase 3: Pluggable Agent Backends Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Extract Claude Code CLI logic from spawner.go into an `AgentRuntime` plugin interface, then implement three plugins: Claude Code, ais-agent, and Aider.

**Architecture:** New `AgentRuntime` interface in `internal/agent/runtime.go` defines the contract. `RuntimeRegistry` manages registered plugins. Each plugin lives in its own subpackage (`claudecode/`, `aisagent/`, `aider/`). `spawner.go` refactored to delegate to plugins via registry. `CompletionMonitor` uses runtime-specific completion detection.

**Tech Stack:** Go 1.23, `tmux.TmuxClient`, interfaces for dependency inversion

**Depends on:** None strictly, but benefits from Phase 1+2 patterns (council and meeting also use runtimes)

---

## Task 1: AgentRuntime Interface & Registry

**Files:**
- Create: `internal/agent/runtime.go`
- Create: `internal/agent/registry.go`
- Create: `internal/agent/registry_test.go`

**Step 1: Write failing tests**

```go
// internal/agent/registry_test.go
func TestRuntimeRegistry_RegisterAndGet(t *testing.T)    // register, get by name
func TestRuntimeRegistry_GetUnknown(t *testing.T)        // unknown name returns false
func TestRuntimeRegistry_Default(t *testing.T)            // first registered is default
func TestRuntimeRegistry_List(t *testing.T)               // returns all names
func TestRuntimeRegistry_ConcurrentAccess(t *testing.T)   // parallel register+get safe
```

**Step 2: Implement runtime.go**

```go
// internal/agent/runtime.go
package agent

type AgentRuntime interface {
    Name() string
    Spawn(ctx context.Context, cfg SpawnConfig) (*AgentSession, error)
    SendPrompt(session *AgentSession, prompt string) error
    CaptureOutput(session *AgentSession, lines int) (string, error)
    DetectReady(ctx context.Context, session *AgentSession, timeout time.Duration) error
    DetectCompletion(ctx context.Context, session *AgentSession) (bool, error)
    ParseTokenUsage(output string) (TokenUsage, error)
    Cleanup(session *AgentSession) error
}

type SpawnConfig struct {
    WorkDir         string
    Branch          string
    SystemPrompt    string
    AllowedTools    []string
    DisallowedTools []string
    Model           string
    PermissionMode  string
    ExtraCLIArgs    string
    EnvVars         map[string]string
}

type AgentSession struct {
    ID          string
    RuntimeName string
    TmuxSession string
    Window      int
    Pane        int
    WorkDir     string
    StartedAt   time.Time
    Metadata    map[string]string
}

type TokenUsage struct {
    InputTokens  int
    OutputTokens int
    TotalCost    float64
}
```

**Step 3: Implement registry.go**

```go
// internal/agent/registry.go
package agent

type RuntimeRegistry struct {
    mu       sync.RWMutex
    runtimes map[string]AgentRuntime
    order    []string  // insertion order for Default()
}

func NewRuntimeRegistry() *RuntimeRegistry
func (r *RuntimeRegistry) Register(rt AgentRuntime)
func (r *RuntimeRegistry) Get(name string) (AgentRuntime, bool)
func (r *RuntimeRegistry) List() []string
func (r *RuntimeRegistry) Default() AgentRuntime  // first registered, or nil
```

**Step 4: Run tests**

Run: `go test ./internal/agent/ -v -race`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/agent/runtime.go internal/agent/registry.go internal/agent/registry_test.go
git commit -m "feat(backends): add AgentRuntime interface and RuntimeRegistry"
```

---

## Task 2: Claude Code Plugin — Extract from Spawner

**Files:**
- Create: `internal/agent/claudecode/runtime.go`
- Create: `internal/agent/claudecode/runtime_test.go`

**Step 1: Write failing tests**

- `TestClaudeCodeRuntime_Name` — returns "claude"
- `TestClaudeCodeRuntime_BuildCLICommand` — verify CLI command construction from SpawnConfig
- `TestClaudeCodeRuntime_ParseReadyIndicators` — test ready detection patterns
- `TestClaudeCodeRuntime_ParseCompletionIndicators` — test completion detection patterns
- `TestClaudeCodeRuntime_ParseTokenUsage` — parse token count from pane output (or return zero)

**Step 2: Implement**

Extract from `spawner.go` (lines 452-612, 802-877):

```go
// internal/agent/claudecode/runtime.go
package claudecode

import (
    "github.com/hanfourmini/aisupervisor/internal/agent"
    "github.com/hanfourmini/aisupervisor/internal/tmux"
)

type Runtime struct {
    tmuxClient tmux.TmuxClient
}

func New(tc tmux.TmuxClient) *Runtime

func (r *Runtime) Name() string { return "claude" }

func (r *Runtime) Spawn(ctx context.Context, cfg agent.SpawnConfig) (*agent.AgentSession, error) {
    // 1. Create tmux session (generate unique name)
    // 2. cd to cfg.WorkDir
    // 3. Unset CLAUDECODE env var
    // 4. Build CLI command: "claude" + permission mode + model + system prompt + tools + extra args
    // 5. Send CLI command to tmux
    // 6. Return AgentSession with tmux coordinates
}

func (r *Runtime) SendPrompt(session *agent.AgentSession, prompt string) error {
    // SendLiteralKeys + render delay + SendKeys Enter
    // Render delay: 1s base + 500ms per 2000 chars, capped at 5s
}

func (r *Runtime) CaptureOutput(session *agent.AgentSession, lines int) (string, error) {
    // tmuxClient.CapturePane with -lines for scrollback
}

func (r *Runtime) DetectReady(ctx context.Context, session *agent.AgentSession, timeout time.Duration) error {
    // Poll every 300ms for:
    //   "❯", "> ", "Welcome back", "What can I help"
    // Auto-accept: "trust" dialog → send "y", skip-permissions dialog → send "y"
}

func (r *Runtime) DetectCompletion(ctx context.Context, session *agent.AgentSession) (bool, error) {
    // Check last non-empty line for idle prompt: ">" or "❯"
    // Require content to have changed 3+ times (prevent false positives)
}

func (r *Runtime) ParseTokenUsage(output string) (agent.TokenUsage, error) {
    // Best-effort: look for token count patterns in output
    // Return zero values if not found (not an error)
}

func (r *Runtime) Cleanup(session *agent.AgentSession) error {
    return r.tmuxClient.KillSession(session.TmuxSession)
}
```

Helper functions (extracted from spawner.go):
- `buildCLICommand(cfg agent.SpawnConfig) string` — construct `claude --model X --permission-mode Y ...`
- `promptRenderDelay(promptLen int) time.Duration` — from spawner line 614
- `isClaudeReady(content string) bool` — from spawner waitForReady
- `isClaudeIdle(content string) bool` — from monitor isClaudeIdle

**Step 3: Run tests**

Run: `go test ./internal/agent/claudecode/ -v`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add internal/agent/claudecode/
git commit -m "feat(backends): add Claude Code CLI runtime plugin"
```

---

## Task 3: ais-agent Plugin

**Files:**
- Create: `internal/agent/aisagent/runtime.go`
- Create: `internal/agent/aisagent/runtime_test.go`

**Step 1: Write failing tests**

- `TestAISAgentRuntime_Name` — returns "ais-agent"
- `TestAISAgentRuntime_BuildCLICommand` — verify ais-agent specific flags
- `TestAISAgentRuntime_ParseReadyIndicators` — ais-agent ready patterns

**Step 2: Implement**

Extract from `spawner.go` `buildAISAgentArgs` (lines 378-406):

```go
// internal/agent/aisagent/runtime.go
package aisagent

type Runtime struct {
    tmuxClient tmux.TmuxClient
}

func New(tc tmux.TmuxClient) *Runtime
func (r *Runtime) Name() string { return "ais-agent" }

func (r *Runtime) Spawn(ctx context.Context, cfg agent.SpawnConfig) (*agent.AgentSession, error) {
    // Similar to Claude but:
    // CLI: "ais-agent --provider anthropic --model {model} --permission-mode {mode}"
    // Tools format: "--allowed-tools tool1,tool2" (comma-separated, not space)
    // Additional: "--max-tokens", "--append-system-prompt"
}

func (r *Runtime) DetectReady(...) error {
    // ais-agent uses similar prompt patterns
    // Poll for ">" or custom ready indicator
}

// Other methods similar to Claude Code, adapted for ais-agent CLI
```

**Step 3: Run tests, commit**

```bash
git commit -m "feat(backends): add ais-agent runtime plugin"
```

---

## Task 4: Aider Plugin

**Files:**
- Create: `internal/agent/aider/runtime.go`
- Create: `internal/agent/aider/runtime_test.go`

**Step 1: Write failing tests**

- `TestAiderRuntime_Name` — returns "aider"
- `TestAiderRuntime_BuildCLICommand` — verify `aider --model X --no-auto-commits --yes`
- `TestAiderRuntime_ParseReadyIndicators` — detect `aider>` prompt
- `TestAiderRuntime_ParseTokenUsage` — parse Aider's "Tokens: X sent, Y received" output
- `TestAiderRuntime_CleanupSendsExit` — verify `/exit` command sent before KillSession

**Step 2: Implement**

```go
// internal/agent/aider/runtime.go
package aider

type Runtime struct {
    tmuxClient tmux.TmuxClient
}

func New(tc tmux.TmuxClient) *Runtime
func (r *Runtime) Name() string { return "aider" }

func (r *Runtime) Spawn(ctx context.Context, cfg agent.SpawnConfig) (*agent.AgentSession, error) {
    // CLI: "aider --model {model} --no-auto-commits --yes"
    // If cfg.SystemPrompt: "--message-file" with temp file containing system prompt
    // Note: Aider doesn't have tool restrictions like Claude Code
}

func (r *Runtime) DetectReady(ctx context.Context, session *agent.AgentSession, timeout time.Duration) error {
    // Poll for "aider>" prompt (case-insensitive)
    // Also accept ">" as fallback
}

func (r *Runtime) DetectCompletion(ctx context.Context, session *agent.AgentSession) (bool, error) {
    // Detect return to "aider>" prompt after activity
    // Content must have changed at least once
}

func (r *Runtime) ParseTokenUsage(output string) (agent.TokenUsage, error) {
    // Parse: "Tokens: 1,234 sent, 567 received, $0.01 cost"
    // Regex: `Tokens:\s*([\d,]+)\s*sent,\s*([\d,]+)\s*received`
}

func (r *Runtime) Cleanup(session *agent.AgentSession) error {
    // Send "/exit" command first (graceful shutdown)
    r.tmuxClient.SendKeys(session.TmuxSession, session.Window, session.Pane, "/exit")
    time.Sleep(2 * time.Second)
    return r.tmuxClient.KillSession(session.TmuxSession)
}
```

**Step 3: Run tests, commit**

```bash
git commit -m "feat(backends): add Aider runtime plugin"
```

---

## Task 5: Spawner Refactoring — Delegate to Plugins

**Files:**
- Modify: `internal/worker/spawner.go`
- Modify: `internal/worker/spawner_test.go` (if exists)

This is the most critical and delicate task — refactoring the existing spawn flow to use plugins.

**Step 1: Add RuntimeRegistry to Spawner**

```go
// Add to Spawner struct
runtimeRegistry *agent.RuntimeRegistry

// Add setter
func (s *Spawner) SetRuntimeRegistry(r *agent.RuntimeRegistry) {
    s.runtimeRegistry = r
}
```

**Step 2: Refactor spawnForTaskInner**

Keep the git branch creation and prompt building in spawner. Delegate CLI operations to runtime:

```go
func (s *Spawner) spawnForTaskInner(ctx context.Context, w *worker.Worker, t *project.Task, p *project.Project) error {
    // --- Git setup (KEEP IN SPAWNER) ---
    // Create branch, setup workdir (lines 453-509 mostly unchanged)

    // --- Runtime selection (NEW) ---
    runtimeName := w.CLITool
    if runtimeName == "" {
        runtimeName = "claude"
    }

    var rt agent.AgentRuntime
    if s.runtimeRegistry != nil {
        rt, _ = s.runtimeRegistry.Get(runtimeName)
    }

    if rt != nil {
        // --- NEW PATH: delegate to runtime plugin ---
        cfg := s.buildSpawnConfig(w, t, p, workDir)
        session, err := rt.Spawn(ctx, cfg)
        if err != nil {
            return fmt.Errorf("runtime %s spawn: %w", runtimeName, err)
        }
        if err := rt.DetectReady(ctx, session, 120*time.Second); err != nil {
            rt.Cleanup(session)
            return fmt.Errorf("runtime %s ready: %w", runtimeName, err)
        }
        prompt := s.buildPromptForTier(t, p, w.EffectiveTier(), deps)
        // ... append personality, knowledge, pending messages (unchanged)
        if err := rt.SendPrompt(session, prompt); err != nil {
            rt.Cleanup(session)
            return fmt.Errorf("runtime %s prompt: %w", runtimeName, err)
        }
        // Update worker from session
        w.TmuxSession = session.TmuxSession
        w.Window = session.Window
        w.Pane = session.Pane
    } else {
        // --- LEGACY PATH: keep existing inline logic as fallback ---
        // (existing lines 511-576 unchanged)
    }

    // --- Session registration (KEEP IN SPAWNER) ---
    // (existing lines 577-611 unchanged)
}
```

**Step 3: Add buildSpawnConfig helper**

```go
func (s *Spawner) buildSpawnConfig(w *worker.Worker, t *project.Task, p *project.Project, workDir string) agent.SpawnConfig {
    skillArgs := s.buildSkillArgs(w)
    // Parse skillArgs back into structured config
    // OR build SpawnConfig directly from skill profile
    profile := s.skillProfiles[w.SkillProfile]
    return agent.SpawnConfig{
        WorkDir:         workDir,
        Branch:          t.BranchName,
        SystemPrompt:    profile.SystemPrompt,
        AllowedTools:    profile.AllowedTools,
        DisallowedTools: profile.DisallowedTools,
        Model:           profile.Model,
        PermissionMode:  profile.PermissionMode,
        ExtraCLIArgs:    profile.ExtraCLIArgs,
    }
}
```

**Step 4: Build and run ALL tests**

Run: `go build ./internal/...`
Run: `go test ./internal/... -count=1`
Expected: ALL PASS — existing behavior preserved via legacy fallback path

**Step 5: Commit**

```bash
git add internal/worker/spawner.go
git commit -m "refactor(backends): delegate spawner CLI operations to AgentRuntime plugins"
```

---

## Task 6: CompletionMonitor — Use Runtime Detection

**Files:**
- Modify: `internal/worker/monitor.go`

**Step 1: Add RuntimeRegistry to monitor**

```go
type CompletionMonitor struct {
    tmuxClient      tmux.TmuxClient
    runtimeRegistry *agent.RuntimeRegistry
}

func (m *CompletionMonitor) SetRuntimeRegistry(r *agent.RuntimeRegistry) {
    m.runtimeRegistry = r
}
```

**Step 2: Integrate into WatchForCompletion**

In the poll loop, add runtime-specific completion detection:

```go
// After capturing pane content, before existing idle detection
if m.runtimeRegistry != nil {
    if rt, ok := m.runtimeRegistry.Get(w.CLITool); ok {
        done, err := rt.DetectCompletion(ctx, &agent.AgentSession{
            TmuxSession: w.TmuxSession,
            Window: w.Window,
            Pane: w.Pane,
        })
        if err == nil && done && changeCount >= 3 {
            return CompletionResult{Success: true, Reason: "runtime_idle"}, nil
        }
    }
}
// Existing detection logic as fallback
```

**Step 3: Build and test**

Run: `go build ./internal/...`
Run: `go test ./internal/worker/ -count=1`

**Step 4: Commit**

```bash
git add internal/worker/monitor.go
git commit -m "refactor(backends): use AgentRuntime for completion detection in monitor"
```

---

## Task 7: Wire Registry in Manager + Update Council/Meeting

**Files:**
- Modify: `internal/company/company.go`
- Modify: `internal/company/council.go`
- Modify: `internal/company/meeting.go` (if Phase 2 done)

**Step 1: Initialize registry in Manager.New()**

```go
// In New(), after tmuxClient is available
runtimeReg := agent.NewRuntimeRegistry()
runtimeReg.Register(claudecode.New(tmuxClient))
runtimeReg.Register(aisagent.New(tmuxClient))
runtimeReg.Register(aider.New(tmuxClient))

spawner.SetRuntimeRegistry(runtimeReg)
monitor.SetRuntimeRegistry(runtimeReg)
```

Add imports:
```go
"github.com/hanfourmini/aisupervisor/internal/agent"
"github.com/hanfourmini/aisupervisor/internal/agent/claudecode"
"github.com/hanfourmini/aisupervisor/internal/agent/aisagent"
"github.com/hanfourmini/aisupervisor/internal/agent/aider"
```

**Step 2: Update council.go spawnExpertAgent**

Replace direct tmux CLI launch with runtime:

```go
func (c *CouncilEngine) spawnExpertAgent(...) ([]ExpertFinding, error) {
    if c.runtimeRegistry == nil {
        return nil, fmt.Errorf("no runtime registry")
    }
    rt, ok := c.runtimeRegistry.Get("claude")
    if !ok {
        rt = c.runtimeRegistry.Default()
    }
    // Use rt.Spawn(), rt.DetectReady(), rt.SendPrompt(), rt.CaptureOutput(), rt.Cleanup()
    // instead of direct tmux calls
}
```

Add `runtimeRegistry *agent.RuntimeRegistry` field to CouncilEngine.

**Step 3: Update meeting.go (if Phase 2 exists)**

Same pattern — CLI mode speech collection uses RuntimeRegistry.

**Step 4: Build and full test**

Run: `go build ./internal/...`
Run: `go test ./internal/... -count=1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/company/company.go internal/company/council.go
git commit -m "feat(backends): wire RuntimeRegistry into Manager, council, and meeting"
```

---

## Task 8: Frontend i18n

**Files:**
- Modify: `frontend/src/lib/stores/i18n.js`

**Step 1: Add translations**

```javascript
// --- Agent Backend Events ---
'settings.runtime': { en: 'Agent Runtime', zh: '代理執行引擎' },
'settings.runtime.claude': { en: 'Claude Code', zh: 'Claude Code' },
'settings.runtime.aisagent': { en: 'AIS Agent', zh: 'AIS Agent' },
'settings.runtime.aider': { en: 'Aider', zh: 'Aider' },
```

**Step 2: Commit**

```bash
git commit -m "feat(backends): add zh-TW translations for runtime settings"
```

---

## Task 9: Integration Test & Final Verification

**Files:**
- Create: `internal/agent/integration_test.go`

**Step 1: Write integration tests**

- `TestRuntimeRegistry_AllPluginsRegistered` — register all 3, verify List() returns 3 names
- `TestClaudeCodeRuntime_BuildCommand_FullConfig` — verify complete CLI command from SpawnConfig
- `TestAiderRuntime_BuildCommand_FullConfig` — verify aider CLI command
- `TestSpawnerWithRegistry_SelectsRuntime` — verify spawner picks correct runtime by CLITool

**Step 2: Final build verification**

Run: `go build ./internal/...`
Run: `go vet ./internal/...`
Run: `go test ./internal/... -count=1`
Expected: ALL PASS

**Step 3: Verify file list**

```
internal/agent/runtime.go          ✓
internal/agent/registry.go         ✓
internal/agent/registry_test.go    ✓
internal/agent/claudecode/runtime.go      ✓
internal/agent/claudecode/runtime_test.go ✓
internal/agent/aisagent/runtime.go        ✓
internal/agent/aisagent/runtime_test.go   ✓
internal/agent/aider/runtime.go           ✓
internal/agent/aider/runtime_test.go      ✓
internal/agent/integration_test.go        ✓
```

**Step 4: Commit**

```bash
git commit -m "test(backends): add integration tests for runtime plugin system"
```
