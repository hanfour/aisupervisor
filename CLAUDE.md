# aisupervisor

AI-powered virtual office supervisor — a Wails v2 desktop app (Go backend + Svelte frontend) that manages AI workers via Claude Code CLI in tmux sessions.

## Tech Stack

- **Backend**: Go 1.23+, Wails v2 (`wails dev` for development)
- **Frontend**: Svelte + Vite (in `frontend/`)
- **AI Workers**: Claude Code CLI running in tmux panes
- **Data**: YAML files in `~/.local/share/aisupervisor/company/`
- **Config**: `~/.config/aisupervisor/config.yaml`
- **Language**: UI is in 繁體中文 (zh-TW), code comments in English

## Architecture

```
cmd/
  aisupervisor-gui/   # Wails v2 GUI entry point (main app)
  aisupervisor/       # TUI entry point (terminal mode)
internal/
  agent/              # AgentRuntime plugin system (Phase 3 backends)
    runtimeutil/      # Shared plugin helpers (ShellEscape, token regex, session names)
    claudecode/       # Claude Code CLI runtime
    aisagent/         # ais-agent runtime
    aider/            # Aider runtime
  ai/                 # Chat provider abstraction (anthropic, openai, ollama, gemini, claudecli)
  company/            # Core business logic — task management, review pipeline, council, meetings, mailbox, chat
  config/             # App config + skill profiles (defaults.go)
  gitops/             # Git branch + worktree operations for task isolation
  gui/                # Wails bindings (CompanyApp — Go↔Svelte bridge)
  knowledge/          # Code graph, convention store, knowledge injection
  messaging/          # Inter-worker messaging primitives
  personality/        # Worker personality traits, skill scores, narratives
  project/            # Project & Task data models
  role/               # AI role system (gatekeeper, resolver)
  supervisor/         # Pane monitoring, activity observation
  tmux/               # tmux client (exec-based, not gotmux)
  worker/             # Worker spawner, monitor, session management
frontend/
  src/lib/
    components/       # Svelte UI components
    office/           # Pixel office simulation
    pages/            # Route pages
    stores/           # Svelte stores + i18n
```

## Key Data Flow

1. **Task Assignment**: GUI → `CompanyApp.AssignTask()` → `company.Manager.AssignTask()` → creates git branch → `spawner.SpawnForTask()` → delegates to `agent.AgentRuntime` plugin (claudecode / aisagent / aider) for tmux session + CLI launch. Legacy inline path preserved as fallback when registry is nil.
2. **Runtime Selection**: `spawner.resolveRuntimeName(w)` picks the runtime by precedence: default `"claude"` → `w.CLITool` → growth config → tier config (last wins). `spawner.buildSpawnConfig` assembles the structured `agent.SpawnConfig` from skill profile + growth + skillOverrides.
3. **Completion Detection**: `monitor.WatchForCompletion()` polls tmux pane; when a registry is wired, prefers `rt.DetectCompletion(ctx, session, content)` (`Reason: "runtime_idle"`) over the legacy `isClaudeIdle` / `isAiderIdle` branches.
4. **Review Pipeline**: task done → status `code_review` → auto-create review sub-task → reviewer completes → `HandleReviewResult()` → `parseReviewVerdict()` (searches for APPROVED/REJECTED in captured pane output).
5. **Council Review**: reviewer can escalate to `CouncilEngine.RunCouncil()` — parallel domain experts (security, performance, testing, …) run via `spawnExpertAgent`, which delegates to the same `AgentRuntime` plugin. Findings pass through a Carmack filter and are summarised into a unified verdict.
6. **Inter-Worker Comms**: `messaging` / `company/mailbox.go` back a YAML-persistent message store; workers emit `ASK:<workerID>:<question>` and `REPLY:<messageID>:<content>` patterns in their pane, detected by the monitor. Pending messages are injected at spawn-time via `spawner.pendingMessagesFn`.
7. **Structured Meetings**: `MeetingEngine` (`company/meeting.go`) runs Review / Planning / Debug meeting scenarios, orchestrating multi-participant chat via `ai.ChatProvider` and persisting minutes in `MeetingStore`.
8. **Skill Profiles**: `config/defaults.go` defines profiles → `spawner.buildSpawnConfig` (runtime path) or `spawner.buildSkillArgs` (legacy path) applies them. Per-worker `skillOverrides` (ExtraPrompt, ModelOverride, AddTools, RemoveTools) are merged on top.

## Development Commands

```bash
# Start dev server (compiles Go + serves frontend)
cd cmd/aisupervisor-gui && /Users/hanfourmini/go/bin/wails dev

# Build for production
wails build

# Run tests
go test ./internal/...

# Dev server URLs
# App: http://localhost:34115
# Frontend HMR: http://localhost:41229
```

## Important Files

| File | Purpose |
|------|---------|
| `internal/config/defaults.go` | Skill profile definitions (system prompts, tool restrictions) |
| `internal/agent/runtime.go` | `AgentRuntime` plugin interface + `SpawnConfig` / `AgentSession` / `TokenUsage` types |
| `internal/agent/registry.go` | `RuntimeRegistry` — thread-safe plugin registry, insertion-order-preserving |
| `internal/agent/runtimeutil/runtimeutil.go` | Shared helpers (ShellEscape, PromptRenderDelay, NewSessionName, token regex) |
| `internal/agent/{claudecode,aisagent,aider}/runtime.go` | Plugin implementations |
| `internal/worker/spawner.go` | Worker spawning; delegates to AgentRuntime plugin, with legacy inline fallback |
| `internal/worker/monitor.go` | Completion detection via tmux polling; runtime-first, legacy fallback |
| `internal/company/company.go` | Core Manager — constructs `RuntimeRegistry`, wires plugins into spawner/monitor/council/meeting |
| `internal/company/review.go` | Review pipeline — verdict parsing, task routing |
| `internal/company/council.go` | Carmack-Council multi-expert review pipeline |
| `internal/company/meeting.go` | MeetingEngine — Review / Planning / Debug scenarios |
| `internal/company/mailbox.go` | Inter-worker mailbox (YAML-backed) |
| `internal/tmux/client.go` | tmux operations (capture-pane with `-S` for scrollback) |
| `internal/gitops/gitops.go` | Git branch, worktree, checkout operations |
| `internal/gui/company_app.go` | Wails bindings for frontend |
| `frontend/src/lib/stores/i18n.js` | UI translations (zh-TW) |

## Known Gotchas

- **tmux capture-pane**: Must use `-S -N` flag for scrollback, otherwise only visible pane is captured
- **SendLiteralKeys + Enter**: `spawner.go` sends prompt via `SendLiteralKeys` then `SendKeys("Enter")` — Enter doesn't always trigger CLI submission
- **YAML errors**: `yaml.Unmarshal` returns errors silently when ignored with `_` — always check errors
- **Wails dev restart**: Code changes require killing wails process and restarting; hot reload only works for frontend
- **Permission mode**: Workers with `bypassPermissions` skip all Claude Code permission prompts; `acceptEdits` auto-accepts file edits but still prompts for Bash
- **Review verdict**: `parseReviewVerdict()` searches last 5000 bytes for "approved"/"rejected" keywords in captured pane output (500 lines scrollback)
- **Autonomous worker skill isolation**: Workers inherit the host's `.claude/` skills (superpowers, brainstorming, etc.) via SessionStart hooks. These interactive skills can override worker prompts and cause infinite loops (brainstorming → planning → task creation). All skill profiles MUST include `Skill`, `EnterPlanMode`, `ExitPlanMode` in `DisallowedTools`. The spawner also enforces `config.AutonomousDisallowedTools()` globally as a safety net.
- **Runtime plugin fallback**: When `Manager.New()` is called with `tmuxClient == nil` (some unit-test paths), the `RuntimeRegistry` is still created but plugin registration is skipped. Both `spawner.spawnForTaskInner` and `monitor.WatchForCompletion` short-circuit to runtime path only when a matching runtime is found; otherwise the legacy inline tmux path runs unchanged.
- **ExtraCLIArgs trust boundary**: `SpawnConfig.ExtraCLIArgs` is appended verbatim to the CLI command without shell escaping. It is treated as trusted config (from SkillProfile / tier YAML) and must never be populated from user input. Runtime plugins document this in their package docstrings.
- **`AIS_PROVIDER` / `AIS_MAX_TOKENS` EnvVars**: ais-agent runtime reads provider + token budget from `cfg.EnvVars` (not from dedicated SpawnConfig fields). The spawner's `buildSpawnConfig` surfaces growth-config values there.
- **Completion detection order**: `monitor.WatchForCompletion` prefers `rt.DetectCompletion(ctx, session, content)` (returns `Reason: "runtime_idle"`) when a registry is wired; falls back to `isClaudeIdle` / `isAiderIdle` when runtime is nil, returns false, or errors (error is logged once per watch call).
- **PixelLab wire format is *only* validated by the live API**: structs in `internal/pixellab/endpoints.go` mirror `https://api.pixellab.ai/v1/openapi.json` and must be in sync with it (field names, label enums, hardcoded counts like the 3-keyframe limit on `animate-with-skeleton`). Unit tests can pass against any shape — they round-trip through Go structs we control. Any change to a request/response struct here requires a live smoke (`GenerateWorkerSprite` end-to-end) before merge. PR #41 caught four such drifts that had passed CI.

## Coding Conventions

- Go: standard `gofmt`, error handling with `fmt.Errorf("context: %w", err)`
- Frontend: Svelte components in `PascalCase.svelte`, stores as JS modules
- Data models: YAML tags on struct fields, JSON tags for Wails bindings
- Tests: `_test.go` files alongside source, table-driven tests preferred
- i18n: All UI strings go through `i18n.js` store, keys like `settings.chatBackend`
