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
  ai/                 # AI backend abstraction (anthropic, openai, ollama, gemini)
  company/            # Core business logic — task management, review pipeline, chat
  config/             # App config + skill profiles (defaults.go)
  gui/                # Wails bindings (CompanyApp — Go↔Svelte bridge)
  personality/        # Worker personality traits, skill scores, narratives
  project/            # Project & Task data models
  role/               # AI role system (gatekeeper, resolver)
  supervisor/         # Pane monitoring, activity observation
  tmux/               # tmux client (exec-based, not gotmux)
  worker/             # Worker spawner, monitor, session management
  gitops/             # Git branch operations for task isolation
frontend/
  src/lib/
    components/       # Svelte UI components
    office/           # Pixel office simulation
    pages/            # Route pages
    stores/           # Svelte stores + i18n
```

## Key Data Flow

1. **Task Assignment**: GUI → `CompanyApp.AssignTask()` → `company.Manager.AssignTask()` → creates git branch → `spawner.SpawnForTask()` → tmux session + CLI
2. **Completion Detection**: `monitor.WatchForCompletion()` polls tmux pane for idle `❯` prompt with `changeCount >= 3`
3. **Review Pipeline**: task done → status `code_review` → auto-create review sub-task → reviewer completes → `HandleReviewResult()` → `parseReviewVerdict()` (searches for APPROVED/REJECTED in captured pane output)
4. **Skill Profiles**: `config/defaults.go` defines profiles → `spawner.buildSkillArgs()` converts to CLI flags (`--append-system-prompt`, `--allowedTools`, `--model`, etc.)

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
| `internal/worker/spawner.go` | Worker spawning, CLI arg building, prompt sending |
| `internal/worker/monitor.go` | Completion detection via tmux polling |
| `internal/company/review.go` | Review pipeline — verdict parsing, task routing |
| `internal/company/company.go` | Core Manager — task assignment, worker management |
| `internal/tmux/client.go` | tmux operations (capture-pane with `-S` for scrollback) |
| `internal/gui/company_app.go` | Wails bindings for frontend |
| `frontend/src/lib/stores/i18n.js` | UI translations (zh-TW) |

## Known Gotchas

- **tmux capture-pane**: Must use `-S -N` flag for scrollback, otherwise only visible pane is captured
- **SendLiteralKeys + Enter**: `spawner.go` sends prompt via `SendLiteralKeys` then `SendKeys("Enter")` — Enter doesn't always trigger CLI submission
- **YAML errors**: `yaml.Unmarshal` returns errors silently when ignored with `_` — always check errors
- **Wails dev restart**: Code changes require killing wails process and restarting; hot reload only works for frontend
- **Permission mode**: Workers with `bypassPermissions` skip all Claude Code permission prompts; `acceptEdits` auto-accepts file edits but still prompts for Bash
- **Review verdict**: `parseReviewVerdict()` searches last 5000 bytes for "approved"/"rejected" keywords in captured pane output (500 lines scrollback)

## Coding Conventions

- Go: standard `gofmt`, error handling with `fmt.Errorf("context: %w", err)`
- Frontend: Svelte components in `PascalCase.svelte`, stores as JS modules
- Data models: YAML tags on struct fields, JSON tags for Wails bindings
- Tests: `_test.go` files alongside source, table-driven tests preferred
- i18n: All UI strings go through `i18n.js` store, keys like `settings.chatBackend`
