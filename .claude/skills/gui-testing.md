---
name: gui-testing
description: End-to-end GUI testing for aisupervisor using Playwright MCP. Use when testing task assignment flow, review pipeline in the UI, worker management, project board interactions, or any browser-based testing of the Wails app at localhost:34115. Also use when the user says "test the GUI", "test in browser", or "e2e test".
---

# GUI Testing with Playwright

Test the aisupervisor Wails desktop app via browser at `http://localhost:34115`.

## Prerequisites

1. Wails dev server running: check `ps aux | grep wails`
2. If not running: `cd cmd/aisupervisor-gui && nohup /Users/hanfourmini/go/bin/wails dev > /tmp/wails-dev.log 2>&1 &`
3. Wait ~40s for compilation, verify with `tail -5 /tmp/wails-dev.log`

## Navigation

The app uses hash-based routing:
- Dashboard: `/#dashboard`
- Projects: `/#projects`
- Board: `/#board` (project task board)
- Workers: `/#workers`
- Terminal: `/#terminal`
- Settings: `/#settings`

Use `browser_navigate` to go directly, or `browser_click` on sidebar buttons.

## Common Test Flows

### Test Task Assignment
1. Navigate to `/#projects`, click into a project
2. Find a task in 就緒 column
3. Click "Assign" button on the task card
4. Select a worker from the radio button list
5. Click 分配 button
6. Verify task moves to 進行中 column
7. Check tmux: `tmux list-sessions` — worker session should appear

### Test Review Pipeline (End-to-End)
1. Assign a coding task to an engineer
2. Wait for completion (monitor tmux pane for idle `❯` prompt)
3. Task should auto-move to 審查中 and create review sub-task
4. Assign review sub-task to a reviewer (Steve, skill: reviewer)
5. Wait for reviewer to complete
6. Check verdict: `tail -20 /tmp/wails-dev.log | grep HandleReviewResult`
7. Verify: if APPROVED, original task should move to 完成

### Test Worker Management
1. Navigate to `/#workers`
2. Click a worker card to see details
3. Verify skill profile, model version, status display correctly

## Playwright Tips

- **Radio buttons**: Click the label container, not the radio input (pointer interception)
- **Snapshot vs Screenshot**: Use `browser_snapshot` for element refs, `browser_take_screenshot` for visual verification
- **Wait for state**: After actions, use `browser_wait_for` with expected text
- **Console errors**: Check `browser_console_messages` for runtime errors
- **Long operations**: tmux worker spawning takes 30-120s, poll with `browser_snapshot` periodically

## Monitoring During Tests

```bash
# Watch wails dev log for errors
tail -f /tmp/wails-dev.log | grep -E "ERR|error|Handle"

# Watch tmux worker activity
tmux capture-pane -t <worker-session> -p -S -50

# Check task status changes
cat ~/.local/share/aisupervisor/company/tasks.yaml | grep -A5 "id: <task-id>"
```

## Troubleshooting

- **Page blank after restart**: Clear browser cache or hard refresh
- **"runtime:ready" errors in log**: Normal Wails v2 behavior, ignore
- **Worker not responding**: Check if tmux session exists, try sending Enter manually
- **Task not updating in UI**: The UI polls — wait a few seconds or refresh the page
