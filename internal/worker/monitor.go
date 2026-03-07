package worker

import (
	"context"
	"strings"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

type CompletionResult struct {
	Success bool
	Reason  string // "idle_prompt", "no_change", "shell_exit"
}

type CompletionMonitor struct {
	tmuxClient tmux.TmuxClient
}

func NewCompletionMonitor(tmuxClient tmux.TmuxClient) *CompletionMonitor {
	return &CompletionMonitor{tmuxClient: tmuxClient}
}

// WatchForCompletion polls the pane content to detect when the CLI tool
// has finished its task. It detects:
// - CLI returning to idle prompt (">")
// - No output change for N consecutive polls
// - Shell prompt appearing (CLI exited)
func (m *CompletionMonitor) WatchForCompletion(ctx context.Context, w *Worker) (CompletionResult, error) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastContent string
	noChangeCount := 0
	const noChangeThreshold = 90 // 90 seconds of no change after meaningful activity
	hadActivity := false
	useAider := w.CLITool == "aider"
	changeCount := 0              // total number of content changes observed
	const minChanges = 3          // require at least 3 content changes before no_change can trigger

	for {
		select {
		case <-ctx.Done():
			return CompletionResult{}, ctx.Err()
		case <-ticker.C:
			content, err := m.tmuxClient.CapturePane(w.TmuxSession, w.Window, w.Pane, 20)
			if err != nil {
				continue
			}

			// Check for shell prompt (CLI has exited)
			if isShellPrompt(content) && hadActivity {
				return CompletionResult{Success: true, Reason: "shell_exit"}, nil
			}

			// Track content changes (must happen before idle checks so
			// changeCount is up-to-date when we evaluate idle_prompt)
			if content == lastContent {
				noChangeCount++
			} else {
				noChangeCount = 0
				hadActivity = true
				changeCount++
			}
			lastContent = content

			// Check for idle prompt based on CLI tool.
			// Require changeCount >= minChanges to avoid triggering on the initial
			// idle prompt before the worker has done any meaningful work.
			if useAider {
				if isAiderIdle(content) && hadActivity && changeCount >= minChanges {
					return CompletionResult{Success: true, Reason: "idle_prompt"}, nil
				}
			} else {
				if isClaudeIdle(content) && hadActivity && changeCount >= minChanges {
					return CompletionResult{Success: true, Reason: "idle_prompt"}, nil
				}
			}

			// Only trigger no_change after enough meaningful activity
			if noChangeCount >= noChangeThreshold && hadActivity && changeCount >= minChanges {
				return CompletionResult{Success: true, Reason: "no_change"}, nil
			}
		}
	}
}

func isClaudeIdle(content string) bool {
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0 && i >= len(lines)-5; i-- {
		trimmed := strings.TrimSpace(lines[i])
		// Claude Code uses ❯ (U+276F) as its prompt character
		if trimmed == ">" || trimmed == "> " || trimmed == "❯" || trimmed == "❯ " {
			return true
		}
	}
	return false
}

// isAiderIdle detects when aider returns to its ">" prompt.
func isAiderIdle(content string) bool {
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0 && i >= len(lines)-5; i-- {
		trimmed := strings.TrimSpace(lines[i])
		// Aider uses ">" as its prompt, sometimes with ANSI codes stripped
		if trimmed == ">" || trimmed == "> " {
			return true
		}
	}
	return false
}

func isShellPrompt(content string) bool {
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0 && i >= len(lines)-3; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		// Common shell prompt patterns
		if strings.HasSuffix(trimmed, "$") || strings.HasSuffix(trimmed, "%") || strings.HasSuffix(trimmed, "#") {
			return true
		}
	}
	return false
}
