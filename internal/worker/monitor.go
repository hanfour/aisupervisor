package worker

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/agent"
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

// gracePeriod is the initial window after monitoring starts during which
// idle-prompt detections are suppressed. It is a variable (not a constant) so
// that tests can temporarily shorten it; restore the original value after use.
var gracePeriod = 30 * time.Second

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

// detectAskPattern scans content for "ASK:{workerID}:{question}" pattern.
// Returns the last occurrence found (most recent in pane output).
func detectAskPattern(content string) (targetID, question string, found bool) {
	lines := strings.Split(content, "\n")
	// Scan from end to start to find the most recent ASK
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "ASK:") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 3)
		if len(parts) != 3 {
			continue
		}
		tid := strings.TrimSpace(parts[1])
		q := strings.TrimSpace(parts[2])
		if tid != "" && q != "" {
			return tid, q, true
		}
	}
	return "", "", false
}

// detectReplyPattern scans content for "REPLY:{messageID}:{content}" pattern.
// Returns the last occurrence found (most recent in pane output).
func detectReplyPattern(content string) (messageID, reply string, found bool) {
	lines := strings.Split(content, "\n")
	// Scan from end to start to find the most recent REPLY
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "REPLY:") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 3)
		if len(parts) != 3 {
			continue
		}
		mid := strings.TrimSpace(parts[1])
		r := strings.TrimSpace(parts[2])
		if mid != "" && r != "" {
			return mid, r, true
		}
	}
	return "", "", false
}

type CompletionResult struct {
	Success     bool
	Reason      string // "idle_prompt", "no_change", "shell_exit", "session_dead", "runtime_idle"
	HelpRequest string // non-empty if HELP_NEEDED: was detected
}

type CompletionMonitor struct {
	tmuxClient      tmux.TmuxClient
	runtimeRegistry *agent.RuntimeRegistry
}

func NewCompletionMonitor(tmuxClient tmux.TmuxClient) *CompletionMonitor {
	return &CompletionMonitor{tmuxClient: tmuxClient}
}

// SetRuntimeRegistry wires a runtime registry so the monitor can delegate
// completion detection to the per-runtime plugin.
func (m *CompletionMonitor) SetRuntimeRegistry(r *agent.RuntimeRegistry) {
	m.runtimeRegistry = r
}

// runtimeIdle consults the matching AgentRuntime (if any) for the worker's
// CLITool and reports whether the runtime considers the session idle, based
// on the pane content the caller has already captured.
//
// Returns (done, err). When no registry is attached or no runtime matches the
// worker's CLITool, it returns (false, nil) so the caller falls back to the
// legacy in-tree detection.
//
// Note: content is passed from the caller's poll-loop capture so the runtime
// does NOT issue a second CapturePane round-trip on each tick.
func (m *CompletionMonitor) runtimeIdle(ctx context.Context, w *Worker, content string) (bool, error) {
	if m.runtimeRegistry == nil {
		return false, nil
	}
	rt, ok := m.runtimeRegistry.Get(w.CLITool)
	if !ok || rt == nil {
		return false, nil
	}
	return rt.DetectCompletion(ctx, &agent.AgentSession{
		TmuxSession: w.TmuxSession,
		Window:      w.Window,
		Pane:        w.Pane,
	}, content)
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
	var lastAskSeen string        // tracks last ASK pattern to avoid re-triggering
	var lastReplySeen string      // tracks last REPLY pattern to avoid re-triggering
	var runtimeErrLogged bool     // one-shot log gate for runtimeIdle errors

	// Grace period: ignore idle prompts for the first N seconds after monitoring starts.
	// This prevents false completion when the CLI briefly shows an idle prompt between
	// receiving the prompt text and starting to process it.
	startTime := time.Now()
	grace := gracePeriod

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

			// Detect ASK request (only trigger on new/different ASK patterns)
			if targetID, question, found := detectAskPattern(content); found {
				askKey := fmt.Sprintf("ASK:%s:%s", targetID, question)
				if askKey != lastAskSeen {
					lastAskSeen = askKey
					return CompletionResult{
						Success:     false,
						Reason:      "ask_request",
						HelpRequest: askKey,
					}, nil
				}
			}

			// Detect REPLY (only trigger on new/different REPLY patterns)
			if msgID, reply, found := detectReplyPattern(content); found {
				replyKey := fmt.Sprintf("REPLY:%s:%s", msgID, reply)
				if replyKey != lastReplySeen {
					lastReplySeen = replyKey
					return CompletionResult{
						Success:     false,
						Reason:      "reply_sent",
						HelpRequest: replyKey,
					}, nil
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
			pastGrace := time.Since(startTime) > grace

			// Prefer runtime-specific completion detection if a registry is wired
			// and a matching runtime is registered for this worker's CLITool. Any
			// error or "not done" result falls through to the legacy branches
			// below so existing behavior is preserved.
			done, derr := m.runtimeIdle(ctx, w, content)
			if derr != nil && !runtimeErrLogged {
				// Log once per WatchForCompletion call to aid debugging without
				// flooding logs; subsequent errors in the same watch silently fall through.
				log.Printf("runtimeIdle[%s cli=%s] error (falling back to legacy detection): %v", w.TmuxSession, w.CLITool, derr)
				runtimeErrLogged = true
			}
			if derr == nil && done && hadActivity && changeCount >= minChanges && pastGrace {
				return CompletionResult{Success: true, Reason: "runtime_idle"}, nil
			}

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

// isClaudeIdle reports whether the captured pane content shows a bare
// Claude Code idle prompt ("❯", "❯ ", ">", "> ") in the trailing
// idleScanLines lines. Claude Code v2+ renders a status bar (⏵⏵ bypass
// permissions, /effort) one to two lines BELOW the prompt, so the last
// non-empty line is not the prompt — a window-based scan is required to
// match the real idle layout. Match is strict (no prefix); "❯ user text"
// means the user is composing input, not idle. Mirrors the helper in
// internal/agent/claudecode/runtime.go (legacy fallback path).
func isClaudeIdle(content string) bool {
	lines := strings.Split(content, "\n")
	start := len(lines) - idleScanLines
	if start < 0 {
		start = 0
	}
	for i := start; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == ">" || trimmed == "> " ||
			trimmed == "❯" || trimmed == "❯ " {
			return true
		}
	}
	return false
}

// idleScanLines mirrors the constant in internal/agent/claudecode/runtime.go.
// Kept duplicated rather than imported to avoid an internal/worker → internal/agent
// dep cycle (worker is a long-standing root pkg; agent is a Phase 3 addition).
const idleScanLines = 5

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
