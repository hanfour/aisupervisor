---
name: debug-pipeline
description: Debug the aisupervisor review pipeline, task assignment, worker spawning, or completion detection. Use when encountering issues with task status transitions, review verdict parsing, tmux capture failures, YAML data corruption, or worker CLI not responding. Also use when the user mentions review pipeline, HandleReviewResult, parseReviewVerdict, WatchForCompletion, or task stuck in wrong status.
---

# Debug Review Pipeline

Systematic approach to debugging the aisupervisor review pipeline and task management system.

## Quick Diagnostics

Run these checks first to identify the problem area:

### 1. Check running state
```bash
# tmux sessions alive?
tmux list-sessions 2>/dev/null || echo "No tmux server"

# Wails dev running?
ps aux | grep wails | grep -v grep

# Dev log errors
tail -50 /tmp/wails-dev.log | grep -E "ERR|error|panic"
```

### 2. Check data integrity
```bash
# Validate YAML files parse correctly
cd ~/.local/share/aisupervisor/company/
for f in *.yaml; do
  python3 -c "import yaml; yaml.safe_load(open('$f'))" 2>&1 && echo "$f: OK" || echo "$f: BROKEN"
done
```

### 3. Check task state
- Read `tasks.yaml` and find the stuck task
- Verify `status`, `reviewer_id`, `review_count`, `parent_task_id` fields
- Check if review sub-tasks exist for `code_review` status tasks

## Common Issues

### Review verdict parsed incorrectly (approved=false when should be true)
**Root cause**: tmux capture-pane not capturing enough scrollback
- Check `internal/tmux/client.go` CapturePane — must use `-S -N` flag
- Check `internal/company/review.go` captureManagerOutput line count (should be 500+)
- Check `parseReviewVerdict` byte search range (should be 5000+)
- **Verify**: `tmux capture-pane -t <session> -p -S -500 | grep -i "approved\|rejected"`

### Worker CLI not submitting prompt
**Root cause**: `SendLiteralKeys` + `SendKeys("Enter")` doesn't always trigger CLI submission
- Check `internal/worker/spawner.go` SpawnForTask around line 209
- **Workaround**: `tmux send-keys -t <session> Enter`
- **Verify**: `tmux capture-pane -t <session> -p | tail -20`

### Task stuck in wrong status
- Check `internal/company/company.go` handleTaskCompletion flow
- For review tasks: verify `parentTaskID` is set correctly
- For code_review tasks: verify review sub-task was created
- **Fix**: Use browser JS API to update task: `window.go.gui.CompanyApp.UpdateTaskStatus(taskID, newStatus)`

### YAML data corruption
- **NEVER** use `yaml.Unmarshal` without checking the error
- **NEVER** write to YAML files while the app is running (app has data in memory)
- To fix: recover data from running app via browser API, then write to file, then restart app

## Key Files Reference

| File | What to check |
|------|--------------|
| `internal/company/review.go` | Review pipeline, verdict parsing, captureManagerOutput |
| `internal/worker/spawner.go` | CLI spawning, prompt sending, skill args |
| `internal/worker/monitor.go` | Completion detection, idle prompt matching |
| `internal/tmux/client.go` | CapturePane scrollback, SendKeys |
| `internal/company/company.go` | Task assignment, status transitions |
| `/tmp/wails-dev.log` | Runtime errors and debug logs |

## Recovery from Browser API

If data files are corrupted but app is still running:
```javascript
// In browser console at localhost:34115
const tasks = await window.go.gui.CompanyApp.ListTasks()
const projects = await window.go.gui.CompanyApp.ListProjects()
const workers = await window.go.gui.CompanyApp.ListWorkers()
```
