package worker

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

// Separate regexes to avoid double-counting (input+output vs total)
var (
	// Matches "Total tokens: 12,345" or "tokens used: 5000" (aggregate totals)
	totalTokensRe = regexp.MustCompile(`(?i)total\s+tokens?(?:\s+used)?[:\s]+([0-9][0-9,_]+)`)
	// Matches "input tokens: 12,345" or "output tokens: 6,789" (individual counts)
	ioTokensRe = regexp.MustCompile(`(?i)(input|output)\s+tokens?[:\s]+([0-9][0-9,_]+)`)
	// Matches Claude Code's cost summary: "Total cost: $0.1234"
	costRe = regexp.MustCompile(`(?i)total\s+cost[:\s]+\$([0-9]+\.?[0-9]*)`)
)

// ParseTokenUsage extracts approximate token usage from Claude Code pane output.
// Prefers "Total tokens" (single value), falls back to summing input+output,
// then estimates from cost. Returns 0 if no usage is found.
func ParseTokenUsage(paneContent string) int64 {
	// Strategy 1: Look for an explicit total (avoids double-counting)
	if matches := totalTokensRe.FindAllStringSubmatch(paneContent, -1); len(matches) > 0 {
		last := matches[len(matches)-1]
		if val := parseTokenNum(last[1]); val > 0 {
			return val
		}
	}

	// Strategy 2: Sum input + output tokens separately
	ioMatches := ioTokensRe.FindAllStringSubmatch(paneContent, -1)
	if len(ioMatches) > 0 {
		var total int64
		for _, m := range ioMatches {
			total += parseTokenNum(m[2])
		}
		if total > 0 {
			return total
		}
	}

	// Strategy 3: Estimate from cost ($3/MTok average)
	if matches := costRe.FindAllStringSubmatch(paneContent, -1); len(matches) > 0 {
		last := matches[len(matches)-1]
		cost, err := strconv.ParseFloat(last[1], 64)
		if err == nil && cost > 0 {
			return int64(cost / 3.0 * 1_000_000)
		}
	}

	return 0
}

func parseTokenNum(s string) int64 {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "_", "")
	val, _ := strconv.ParseInt(s, 10, 64)
	return val
}

type CompletionResult struct {
	Success     bool
	Reason      string // "idle_prompt", "no_change", "shell_exit"
	HelpRequest string // non-empty if HELP_NEEDED: was detected
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
	noChangeThreshold := noChangeThresholdForModel(w.ModelVersion) // seconds of no change after meaningful activity
	hadActivity := false
	useAider := w.CLITool == "aider"
	changeCount := 0              // total number of content changes observed
	const minChanges = 3          // require at least 3 content changes before no_change can trigger
	captureErrors := 0            // consecutive CapturePane failures
	const maxCaptureErrors = 30   // after 30 consecutive errors, check if session is dead

	// Grace period: ignore idle prompts for the first N seconds after monitoring starts.
	// This prevents false completion when the CLI briefly shows an idle prompt between
	// receiving the prompt text and starting to process it.
	startTime := time.Now()
	const gracePeriod = 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return CompletionResult{}, ctx.Err()
		case <-ticker.C:
			content, err := m.tmuxClient.CapturePane(w.TmuxSession, w.Window, w.Pane, 20)
			if err != nil {
				captureErrors++
				if captureErrors >= maxCaptureErrors {
					// Check if tmux session still exists
					if exists, _ := m.tmuxClient.HasSession(w.TmuxSession); !exists {
						return CompletionResult{Success: false, Reason: "session_dead"}, nil
					}
					captureErrors = 0 // session exists, reset counter
				}
				continue
			}
			captureErrors = 0

			// Check for shell prompt (CLI has exited)
			if isShellPrompt(content) && hadActivity {
				return CompletionResult{Success: true, Reason: "shell_exit"}, nil
			}

			// Check for help request keyword (only if newly appeared, not in previous content)
			if strings.Contains(content, "HELP_NEEDED:") && !strings.Contains(lastContent, "HELP_NEEDED:") {
				if helpIdx := strings.Index(content, "HELP_NEEDED:"); helpIdx != -1 {
					helpContent := content[helpIdx+len("HELP_NEEDED:"):]
					if nlIdx := strings.Index(helpContent, "\n"); nlIdx != -1 {
						helpContent = helpContent[:nlIdx]
					}
					helpContent = strings.TrimSpace(helpContent)
					if helpContent != "" {
						return CompletionResult{Success: false, Reason: "help_needed", HelpRequest: helpContent}, nil
					}
				}
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
			// Also skip during grace period to avoid false completion when CLI
			// briefly shows idle prompt between receiving and processing the prompt.
			pastGrace := time.Since(startTime) > gracePeriod
			if useAider {
				if isAiderIdle(content) && hadActivity && changeCount >= minChanges && pastGrace {
					return CompletionResult{Success: true, Reason: "idle_prompt"}, nil
				}
			} else {
				if isClaudeIdle(content) && hadActivity && changeCount >= minChanges && pastGrace {
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
	// Find the last non-empty line. The idle prompt ❯ must be the very last
	// non-empty content — not just appearing anywhere in the last 5 lines.
	// This prevents false positives from ❯ in interactive selection menus
	// or in the prompt input display (e.g. "❯ some user input").
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		// Claude Code uses ❯ (U+276F) as its prompt character
		return trimmed == ">" || trimmed == "> " || trimmed == "❯" || trimmed == "❯ "
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

// noChangeThresholdForModel returns the no-change timeout in seconds based on model.
// Larger/slower models get longer timeouts to avoid premature completion.
func noChangeThresholdForModel(model string) int {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "opus"):
		return 180 // 3 minutes for opus
	case strings.Contains(m, "sonnet"):
		return 120 // 2 minutes for sonnet
	case strings.Contains(m, "haiku"):
		return 60 // 1 minute for haiku
	default:
		return 90 // default
	}
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
