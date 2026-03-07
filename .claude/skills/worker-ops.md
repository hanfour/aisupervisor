---
name: worker-ops
description: Manage aisupervisor AI workers — spawning, monitoring, tmux sessions, skill profiles, and task lifecycle. Use when working on spawner.go, monitor.go, worker management, tmux integration, skill profile configuration, or when the user mentions worker spawning, completion detection, idle detection, or CLI tool configuration.
---

# Worker Operations

Guide for developing and debugging the aisupervisor worker system.

## Architecture Overview

```
Worker Assignment Flow:
  GUI → CompanyApp.AssignTask()
    → company.Manager.AssignTask()
      → gitops: create branch ai/<project>/<task>
      → spawner.SpawnForTask()
        → tmux: create session
        → resolveCLI() + buildSkillArgs() → CLI command
        → waitForReady() → detect CLI ready prompt
        → sendPrompt() via SendLiteralKeys + SendKeys("Enter")
      → monitor.WatchForCompletion() (goroutine)
        → poll tmux pane every 1s
        → detect idle ❯ with changeCount >= 3
        → handleTaskCompletion()
```

## Key Components

### Spawner (`internal/worker/spawner.go`)
- `SpawnForTask()`: Main entry — creates tmux session, starts CLI, sends prompt
- `resolveCLI()`: Determines CLI tool, args, and ready-check regex
- `buildSkillArgs()`: Converts SkillProfile to CLI flags
- `waitForReady()`: Polls tmux for CLI ready prompt (timeout: 120s)
- `buildPromptForTier()`: Builds task prompt based on worker tier

### Monitor (`internal/worker/monitor.go`)
- `WatchForCompletion()`: Polls tmux pane for idle state
- Detects completion when CLI shows `❯` prompt and content stops changing
- `changeCount >= minChanges (3)`: Prevents premature detection
- Captures 20 lines of pane for polling

### Skill Profiles (`internal/config/defaults.go`)
8 built-in profiles: coder, hacker, designer, analyst, architect, devops, reviewer, researcher

Each profile maps to CLI flags:
- `SystemPrompt` → `--append-system-prompt`
- `AllowedTools` → `--allowedTools`
- `DisallowedTools` → `--disallowedTools`
- `Model` → `--model`
- `PermissionMode` → `--dangerously-skip-permissions` or `--permission-mode`

### tmux Client (`internal/tmux/client.go`)
- Exec-based (not gotmux library — avoids socket visibility issues)
- `CapturePane(session, window, pane, lines)`: Uses `-S -N` for scrollback
- `SendKeys()`: Handles trailing special keys (Enter, Escape, etc.)
- `SendLiteralKeys()`: Sends raw text with `-l` flag

## Adding a New Skill Profile

1. Add to `DefaultSkillProfiles()` in `internal/config/defaults.go`
2. Or add to `config.yaml` under `skill_profiles:` (overrides defaults by ID)

```yaml
# config.yaml
skill_profiles:
  - id: my-profile
    name: My Profile
    description: What this profile does
    icon: "\U0001F4BB"
    system_prompt: "You are..."
    allowed_tools: ["Read", "Edit", "Bash"]
    permission_mode: "acceptEdits"
    model: "sonnet"
```

## Debugging Worker Issues

### Worker won't start
```bash
# Check tmux session exists
tmux has-session -t aiworker-<worker-id> 2>/dev/null && echo "exists" || echo "missing"

# Check what's in the pane
tmux capture-pane -t aiworker-<worker-id> -p -S -100

# Check wails log for spawn errors
grep "spawn\|ready\|error" /tmp/wails-dev.log | tail -20
```

### Completion not detected
- Verify `❯` (U+276F) is the idle prompt character in monitor.go
- Check `changeCount` threshold — must be >= 3 consecutive identical captures
- Ensure worker actually finished (check pane output)

### Prompt not submitted
- Known issue: `SendLiteralKeys` + `SendKeys("Enter")` doesn't always work
- Manual fix: `tmux send-keys -t aiworker-<id> Enter`

## Testing Changes

```bash
# Build only (fast check)
go build ./internal/worker/ ./internal/config/

# Run tests
go test ./internal/worker/... ./internal/config/...

# Full app rebuild requires wails dev restart
kill $(ps aux | grep wails | grep -v grep | awk '{print $2}')
cd cmd/aisupervisor-gui && nohup /Users/hanfourmini/go/bin/wails dev > /tmp/wails-dev.log 2>&1 &
```
