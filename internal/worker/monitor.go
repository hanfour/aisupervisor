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
	const noChangeThreshold = 30 // 30 seconds of no change after initial activity
	hadActivity := false
	useAider := w.CLITool == "aider"

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

			// Check for idle prompt based on CLI tool
			if useAider {
				if isAiderIdle(content) && hadActivity {
					return CompletionResult{Success: true, Reason: "idle_prompt"}, nil
				}
			} else {
				if isClaudeIdle(content) && hadActivity {
					return CompletionResult{Success: true, Reason: "idle_prompt"}, nil
				}
			}

			// Track content changes
			if content == lastContent {
				noChangeCount++
			} else {
				noChangeCount = 0
				hadActivity = true
			}
			lastContent = content

			if noChangeCount >= noChangeThreshold && hadActivity {
				return CompletionResult{Success: true, Reason: "no_change"}, nil
			}
		}
	}
}

func isClaudeIdle(content string) bool {
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0 && i >= len(lines)-5; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == ">" || trimmed == "> " {
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
