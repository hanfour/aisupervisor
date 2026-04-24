// Package claudecode implements the agent.AgentRuntime interface for the
// Claude Code CLI. The runtime drives a `claude` process inside a tmux pane
// and exposes lifecycle operations (spawn, prompt, capture, ready/completion
// detection, token parsing, cleanup) on a uniform contract.
//
// The helpers in this file (buildCLICommand, isClaudeReady, isClaudeIdle,
// shellEscape, token-parsing regexes, promptRenderDelay) mirror the
// longer-standing implementations in internal/worker/spawner.go and
// internal/worker/monitor.go. The duplication is intentional for Phase 3
// Task 2 — Task 5 will revisit consolidation once the spawner has been
// refactored to consume runtimes instead of open-coding these details.
package claudecode

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/agent"
	"github.com/hanfourmini/aisupervisor/internal/agent/runtimeutil"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

// Compile-time assertion that *Runtime satisfies agent.AgentRuntime.
var _ agent.AgentRuntime = (*Runtime)(nil)

// Runtime drives the Claude Code CLI inside a tmux pane.
type Runtime struct {
	tmuxClient tmux.TmuxClient
}

// New returns a Runtime backed by the supplied tmux client. The tmux client is
// allowed to be nil for callers that only exercise the pure helpers (for
// example, unit tests that poke at Name() or ParseTokenUsage).
func New(tc tmux.TmuxClient) *Runtime {
	return &Runtime{tmuxClient: tc}
}

// Name returns the stable identifier for this runtime.
func (r *Runtime) Name() string { return "claude" }

// MonitoredSessionType returns the tool-type tag used by supervisor.MonitoredSession.
func (r *Runtime) MonitoredSessionType() string { return "claude_code" }

// Spawn creates a new tmux session, primes the shell, and launches the Claude
// Code CLI using flags derived from cfg. The returned session handle does NOT
// imply the CLI is ready to accept prompts — callers must follow up with
// DetectReady before sending anything via SendPrompt.
func (r *Runtime) Spawn(ctx context.Context, cfg agent.SpawnConfig) (*agent.AgentSession, error) {
	if r.tmuxClient == nil {
		return nil, fmt.Errorf("claudecode: tmux client is nil")
	}

	name, err := runtimeutil.NewSessionName("claude")
	if err != nil {
		return nil, fmt.Errorf("generating session name: %w", err)
	}

	if err := r.tmuxClient.CreateSession(name); err != nil {
		return nil, fmt.Errorf("creating tmux session %q: %w", name, err)
	}

	// If any subsequent setup step fails we best-effort tear the session down.
	cleanup := func(reason error) (*agent.AgentSession, error) {
		if killErr := r.tmuxClient.KillSession(name); killErr != nil {
			return nil, fmt.Errorf("%w (also failed to kill session: %v)", reason, killErr)
		}
		return nil, reason
	}

	// 1. cd into the working directory (if provided) so the CLI inherits it.
	if cfg.WorkDir != "" {
		if err := r.tmuxClient.SendKeys(name, 0, 0, fmt.Sprintf("cd %s", runtimeutil.ShellEscape(cfg.WorkDir))+" Enter"); err != nil {
			return cleanup(fmt.Errorf("sending cd: %w", err))
		}
		time.Sleep(300 * time.Millisecond)
	}

	// 2. Unset CLAUDECODE to avoid the "nested session" guard when the
	// supervisor itself is running inside a Claude Code session.
	if err := r.tmuxClient.SendKeys(name, 0, 0, "unset CLAUDECODE"+" Enter"); err != nil {
		return cleanup(fmt.Errorf("sending unset: %w", err))
	}
	time.Sleep(200 * time.Millisecond)

	// 3. Launch the CLI with the composed flags.
	cmdLine := buildCLICommand(cfg)
	if err := r.tmuxClient.SendKeys(name, 0, 0, cmdLine+" Enter"); err != nil {
		return cleanup(fmt.Errorf("sending CLI command: %w", err))
	}

	return &agent.AgentSession{
		RuntimeName: r.Name(),
		TmuxSession: name,
		Window:      0,
		Pane:        0,
		WorkDir:     cfg.WorkDir,
		StartedAt:   time.Now(),
		Metadata:    map[string]string{},
	}, nil
}

// SendPrompt pastes prompt into the pane, waits briefly for it to render, and
// then submits via Enter. The render delay scales with prompt length.
func (r *Runtime) SendPrompt(session *agent.AgentSession, prompt string) error {
	if r.tmuxClient == nil {
		return fmt.Errorf("claudecode: tmux client is nil")
	}
	if session == nil {
		return fmt.Errorf("claudecode: nil session")
	}
	if err := r.tmuxClient.SendLiteralKeys(session.TmuxSession, session.Window, session.Pane, prompt); err != nil {
		return fmt.Errorf("sending literal keys: %w", err)
	}
	time.Sleep(runtimeutil.PromptRenderDelay(len(prompt)))
	if err := r.tmuxClient.SendKeys(session.TmuxSession, session.Window, session.Pane, "Enter"); err != nil {
		return fmt.Errorf("sending Enter: %w", err)
	}
	return nil
}

// CaptureOutput returns the last `lines` lines of the pane (including
// scrollback when supported by the tmux client).
func (r *Runtime) CaptureOutput(session *agent.AgentSession, lines int) (string, error) {
	if r.tmuxClient == nil {
		return "", fmt.Errorf("claudecode: tmux client is nil")
	}
	if session == nil {
		return "", fmt.Errorf("claudecode: nil session")
	}
	return r.tmuxClient.CapturePane(session.TmuxSession, session.Window, session.Pane, lines)
}

// DetectReady polls the pane until isClaudeReady returns true or the timeout
// elapses. Along the way it auto-accepts two known confirmation dialogs:
//   - Trust-folder dialog: answer "Yes, I trust this folder" (Enter only).
//   - Skip-permissions dialog: move selection Down then Enter.
func (r *Runtime) DetectReady(ctx context.Context, session *agent.AgentSession, timeout time.Duration) error {
	if r.tmuxClient == nil {
		return fmt.Errorf("claudecode: tmux client is nil")
	}
	if session == nil {
		return fmt.Errorf("claudecode: nil session")
	}

	deadline := time.After(timeout)
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	pollCount := 0
	bypassAccepted := false
	bypassAcceptedAt := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("claudecode: timeout waiting for CLI ready after %d polls", pollCount)
		case <-ticker.C:
			pollCount++
			content, err := r.tmuxClient.CapturePane(session.TmuxSession, session.Window, session.Pane, 10)
			if err != nil {
				continue
			}

			// Auto-accept dialogs.
			if !bypassAccepted && (strings.Contains(content, "No, exit") || strings.Contains(content, "I accept")) {
				if strings.Contains(content, "I trust this folder") {
					// Trust folder dialog: "Yes, I trust" is option 1 — Enter is enough.
					_ = r.tmuxClient.SendKeys(session.TmuxSession, session.Window, session.Pane, "Enter")
				} else {
					// Skip-permissions dialog: Down then Enter to pick "Yes, I accept".
					_ = r.tmuxClient.SendKeys(session.TmuxSession, session.Window, session.Pane, "Down")
					time.Sleep(300 * time.Millisecond)
					_ = r.tmuxClient.SendKeys(session.TmuxSession, session.Window, session.Pane, "Enter")
				}
				bypassAccepted = true
				bypassAcceptedAt = pollCount
				continue
			}

			// Skip ready detection while the dialog text is still in the pane buffer.
			if strings.Contains(content, "No, exit") || strings.Contains(content, "I accept") {
				if !bypassAccepted {
					continue
				}
				if pollCount-bypassAcceptedAt < 10 {
					continue
				}
			}

			if isClaudeReady(content) {
				return nil
			}
		}
	}
}

// DetectCompletion is a pure check over already-captured pane content:
// returns true iff the last non-empty line is a bare Claude Code idle prompt
// ("❯", ">"). Callers own the pane-capture cadence; passing stale or empty
// content is valid and returns false.
func (r *Runtime) DetectCompletion(ctx context.Context, session *agent.AgentSession, content string) (bool, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return false, err
		}
	}
	return isClaudeIdle(content), nil
}

// ParseTokenUsage does a best-effort extraction of token/cost metrics from the
// given output. Any absence of data is considered a successful "nothing to
// report" rather than an error.
//
// Convention: when only an aggregate "Total tokens" value is found, it is
// written to InputTokens and OutputTokens is left at 0. This preserves the
// total (InputTokens + OutputTokens == reported total) without inventing a
// fake breakdown.
func (r *Runtime) ParseTokenUsage(output string) (agent.TokenUsage, error) {
	var usage agent.TokenUsage

	// Strategy 1: explicit aggregate "Total tokens" value.
	if matches := runtimeutil.TotalTokensRe.FindAllStringSubmatch(output, -1); len(matches) > 0 {
		last := matches[len(matches)-1]
		if val := runtimeutil.ParseTokenNum(last[1]); val > 0 {
			usage.InputTokens = val
		}
	}

	// Strategy 2: sum individual input/output token lines when no aggregate.
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		if ioMatches := runtimeutil.IOTokensRe.FindAllStringSubmatch(output, -1); len(ioMatches) > 0 {
			for _, m := range ioMatches {
				val := runtimeutil.ParseTokenNum(m[2])
				if val == 0 {
					continue
				}
				switch strings.ToLower(m[1]) {
				case "input":
					usage.InputTokens += val
				case "output":
					usage.OutputTokens += val
				}
			}
		}
	}

	// Strategy 3: cost, additive regardless of whether tokens were found.
	if matches := runtimeutil.CostRe.FindAllStringSubmatch(output, -1); len(matches) > 0 {
		last := matches[len(matches)-1]
		if cost, err := strconv.ParseFloat(last[1], 64); err == nil && cost > 0 {
			usage.TotalCost = cost
		}
	}

	return usage, nil
}

// Cleanup terminates the tmux session owned by the given handle.
func (r *Runtime) Cleanup(session *agent.AgentSession) error {
	if r.tmuxClient == nil {
		return fmt.Errorf("claudecode: tmux client is nil")
	}
	if session == nil {
		return fmt.Errorf("claudecode: nil session")
	}
	return r.tmuxClient.KillSession(session.TmuxSession)
}

// -------------------------------------------------------------------------
// Unexported helpers
// -------------------------------------------------------------------------

// buildCLICommand produces the full "claude <flags>" invocation.
//
// Flag order, chosen to read top-to-bottom from most important to least:
//  1. --model
//  2. --permission-mode / --dangerously-skip-permissions
//  3. --allowedTools ...
//  4. --disallowedTools ...
//  5. --append-system-prompt <escaped>
//  6. ExtraCLIArgs (verbatim, caller-controlled)
func buildCLICommand(cfg agent.SpawnConfig) string {
	parts := []string{"claude"}

	if cfg.Model != "" {
		parts = append(parts, "--model", cfg.Model)
	}

	if cfg.PermissionMode != "" {
		if cfg.PermissionMode == "bypassPermissions" {
			parts = append(parts, "--dangerously-skip-permissions")
		} else {
			parts = append(parts, "--permission-mode", cfg.PermissionMode)
		}
	}

	if len(cfg.AllowedTools) > 0 {
		parts = append(parts, "--allowedTools")
		for _, tool := range cfg.AllowedTools {
			parts = append(parts, runtimeutil.ShellEscape(tool))
		}
	}

	if len(cfg.DisallowedTools) > 0 {
		parts = append(parts, "--disallowedTools")
		for _, tool := range cfg.DisallowedTools {
			parts = append(parts, runtimeutil.ShellEscape(tool))
		}
	}

	if cfg.SystemPrompt != "" {
		parts = append(parts, "--append-system-prompt", runtimeutil.ShellEscape(cfg.SystemPrompt))
	}

	// cfg.ExtraCLIArgs is a trusted config value (sourced from SkillProfile /
	// tier YAML, never from user input). It is appended verbatim without
	// shell escaping so callers can pass pre-formed flag strings.
	if cfg.ExtraCLIArgs != "" {
		parts = append(parts, cfg.ExtraCLIArgs)
	}

	return strings.Join(parts, " ")
}

// isClaudeReady reports whether the captured pane content indicates the
// Claude Code CLI is ready for input. A match fires on either:
//
//  1. The content contains the startup banner ("What can I help" or
//     "Welcome back") — anywhere in the buffer. Banners only appear on
//     CLI launch so this cannot false-fire after the first prompt.
//  2. The LAST non-empty line is a bare prompt (">", "> ", "❯", "❯ ") or
//     starts with "> " / "❯ " (user has begun typing at the prompt).
//
// The last-non-empty-line rule (matching isClaudeIdle) prevents false
// positives when "> quoted text" or similar prompt-looking content appears
// earlier in the scrollback from previous turns.
func isClaudeReady(content string) bool {
	if strings.Contains(content, "What can I help") || strings.Contains(content, "Welcome back") {
		return true
	}
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		return trimmed == ">" || trimmed == "> " ||
			trimmed == "❯" || trimmed == "❯ " ||
			strings.HasPrefix(trimmed, "> ") ||
			strings.HasPrefix(trimmed, "❯ ")
	}
	return false
}

// isClaudeIdle reports whether the LAST non-empty line of content is a bare
// Claude Code idle prompt. This avoids false positives when "❯" appears
// earlier in the pane buffer followed by user-entered text.
func isClaudeIdle(content string) bool {
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		return trimmed == ">" || trimmed == "> " || trimmed == "❯" || trimmed == "❯ "
	}
	return false
}

