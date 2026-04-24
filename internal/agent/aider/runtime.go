// Package aider implements the agent.AgentRuntime interface for the Aider CLI
// (https://aider.chat). The runtime drives an `aider` process inside a tmux
// pane and exposes lifecycle operations (spawn, prompt, capture, ready/
// completion detection, token parsing, cleanup) on a uniform contract.
//
// Conventions that differ from other runtimes:
//
//   - Aider has NO concept of --allowed-tools, --disallowed-tools,
//     --permission-mode, or --dangerously-skip-permissions. Those fields on
//     SpawnConfig are silently ignored; aider relies on file-level git
//     confirmation and the --yes flag for interactive prompts.
//   - --no-auto-commits is ALWAYS emitted: gitops owns git in this project and
//     we do not want aider to race it.
//   - --yes is ALWAYS emitted: workers run non-interactively; every aider
//     confirmation must auto-accept.
//   - SystemPrompt is delivered via a temporary file (--message-file) because
//     aider accepts a single startup message but not a persistent system
//     prompt. The file is recorded in Metadata["systemPromptFile"] and is
//     removed on Cleanup() AFTER the /exit sleep, so aider has had time to
//     consume it.
//   - Idle prompt is ">" (sometimes "aider>"). Because the bare ">" is weak
//     signal, isAiderIdle requires the prompt to be the LAST non-empty line.
//
// The helpers (buildCLICommand, isAiderReady, isAiderIdle, shellEscape,
// promptRenderDelay, parseTokenNum) are duplicated from the sibling claudecode
// and aisagent packages. Phase 3 Task 5 is expected to consolidate them.
package aider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/agent"
	"github.com/hanfourmini/aisupervisor/internal/agent/runtimeutil"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

// Compile-time assertion that *Runtime satisfies agent.AgentRuntime.
var _ agent.AgentRuntime = (*Runtime)(nil)

// Runtime drives the aider CLI inside a tmux pane.
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
func (r *Runtime) Name() string { return "aider" }

// MonitoredSessionType returns the tool-type tag used by supervisor.MonitoredSession.
func (r *Runtime) MonitoredSessionType() string { return "aider" }

// Spawn creates a new tmux session, primes the shell, and launches aider using
// flags derived from cfg. When cfg.SystemPrompt is non-empty the prompt is
// written to a temp file and forwarded via --message-file; the path is stored
// under session.Metadata["systemPromptFile"] so Cleanup can remove it AFTER
// aider has consumed it.
//
// The returned session does NOT imply aider is ready; callers must follow up
// with DetectReady before SendPrompt.
func (r *Runtime) Spawn(ctx context.Context, cfg agent.SpawnConfig) (*agent.AgentSession, error) {
	if r.tmuxClient == nil {
		return nil, fmt.Errorf("aider: tmux client is nil")
	}

	name, err := runtimeutil.NewSessionName("aider")
	if err != nil {
		return nil, fmt.Errorf("generating session name: %w", err)
	}

	if err := r.tmuxClient.CreateSession(name); err != nil {
		return nil, fmt.Errorf("creating tmux session %q: %w", name, err)
	}

	// Best-effort teardown if any subsequent setup step fails.
	cleanup := func(reason error, tmpFile string) (*agent.AgentSession, error) {
		if tmpFile != "" {
			_ = os.Remove(tmpFile)
		}
		if killErr := r.tmuxClient.KillSession(name); killErr != nil {
			return nil, fmt.Errorf("%w (also failed to kill session: %v)", reason, killErr)
		}
		return nil, reason
	}

	// 1. cd into the working directory so the CLI inherits it.
	if cfg.WorkDir != "" {
		if err := r.tmuxClient.SendKeys(name, 0, 0, fmt.Sprintf("cd %s", runtimeutil.ShellEscape(cfg.WorkDir))+" Enter"); err != nil {
			return cleanup(fmt.Errorf("sending cd: %w", err), "")
		}
		time.Sleep(300 * time.Millisecond)
	}

	// 2. If a system prompt is set, stage it in a temp file. We do NOT delete
	// the file here; aider consumes it at startup but Cleanup removes it later.
	var systemPromptFile string
	if cfg.SystemPrompt != "" {
		f, err := os.CreateTemp("", "aider-prompt-*.md")
		if err != nil {
			return cleanup(fmt.Errorf("creating prompt temp file: %w", err), "")
		}
		if _, werr := f.WriteString(cfg.SystemPrompt); werr != nil {
			_ = f.Close()
			return cleanup(fmt.Errorf("writing prompt temp file: %w", werr), f.Name())
		}
		if cerr := f.Close(); cerr != nil {
			return cleanup(fmt.Errorf("closing prompt temp file: %w", cerr), f.Name())
		}
		systemPromptFile = f.Name()
	}

	// 3. Launch aider with the composed flags.
	cmdLine := buildCLICommand(cfg, systemPromptFile)
	if err := r.tmuxClient.SendKeys(name, 0, 0, cmdLine+" Enter"); err != nil {
		return cleanup(fmt.Errorf("sending CLI command: %w", err), systemPromptFile)
	}

	meta := map[string]string{}
	if systemPromptFile != "" {
		meta["systemPromptFile"] = systemPromptFile
	}

	return &agent.AgentSession{
		RuntimeName: r.Name(),
		TmuxSession: name,
		Window:      0,
		Pane:        0,
		WorkDir:     cfg.WorkDir,
		StartedAt:   time.Now(),
		Metadata:    meta,
	}, nil
}

// SendPrompt pastes prompt into the pane, waits briefly for it to render, and
// then submits via Enter. The render delay scales with prompt length.
func (r *Runtime) SendPrompt(session *agent.AgentSession, prompt string) error {
	if r.tmuxClient == nil {
		return fmt.Errorf("aider: tmux client is nil")
	}
	if session == nil {
		return fmt.Errorf("aider: nil session")
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
		return "", fmt.Errorf("aider: tmux client is nil")
	}
	if session == nil {
		return "", fmt.Errorf("aider: nil session")
	}
	return r.tmuxClient.CapturePane(session.TmuxSession, session.Window, session.Pane, lines)
}

// DetectReady polls the pane until isAiderReady returns true or the timeout
// elapses. Unlike claudecode there is no trust-folder / skip-permissions
// dialog to auto-accept — aider's --yes flag handles interactive prompts
// further down the line.
func (r *Runtime) DetectReady(ctx context.Context, session *agent.AgentSession, timeout time.Duration) error {
	if r.tmuxClient == nil {
		return fmt.Errorf("aider: tmux client is nil")
	}
	if session == nil {
		return fmt.Errorf("aider: nil session")
	}

	deadline := time.After(timeout)
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	pollCount := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("aider: timeout waiting for CLI ready after %d polls", pollCount)
		case <-ticker.C:
			pollCount++
			content, err := r.tmuxClient.CapturePane(session.TmuxSession, session.Window, session.Pane, 10)
			if err != nil {
				continue
			}
			if isAiderReady(content) {
				return nil
			}
		}
	}
}

// DetectCompletion is a pure check over already-captured pane content:
// returns true iff the last non-empty line is a bare aider idle prompt
// ("aider>" case-insensitive, or ">"). Callers (the monitor) are expected
// to require a minimum changeCount before trusting this — a single bare
// ">" in the buffer is weak signal on its own.
func (r *Runtime) DetectCompletion(ctx context.Context, session *agent.AgentSession, content string) (bool, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return false, err
		}
	}
	return isAiderIdle(content), nil
}

// ParseTokenUsage extracts token counts and cost from aider's telemetry line:
//
//	"Tokens: 1,234 sent, 567 received, $0.01 cost"
//
// Cost is optional; if absent, TotalCost is left at 0. Missing pattern is not
// an error — zero value is returned so the caller can treat it as "nothing to
// report".
func (r *Runtime) ParseTokenUsage(output string) (agent.TokenUsage, error) {
	var usage agent.TokenUsage

	matches := aiderTokensRe.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return usage, nil
	}

	// Take the last match so late-run summaries win over earlier ones.
	last := matches[len(matches)-1]

	usage.InputTokens = runtimeutil.ParseTokenNum(last[1])
	usage.OutputTokens = runtimeutil.ParseTokenNum(last[2])

	// Group 3 (cost) is optional.
	if len(last) >= 4 && last[3] != "" {
		if cost, err := strconv.ParseFloat(last[3], 64); err == nil && cost > 0 {
			usage.TotalCost = cost
		}
	}

	return usage, nil
}

// Cleanup tears down the aider session. Order of operations:
//  1. SendKeys "/exit" + Enter so aider flushes transcripts / final telemetry.
//  2. Sleep 2s to let aider actually exit before we yank the tmux session.
//  3. KillSession.
//  4. Remove the temp system-prompt file (best-effort).
//
// All steps are attempted even on partial failure; the first non-nil error
// (or a composite of all encountered errors) is returned.
func (r *Runtime) Cleanup(session *agent.AgentSession) error {
	if r.tmuxClient == nil {
		return fmt.Errorf("aider: tmux client is nil")
	}
	if session == nil {
		return fmt.Errorf("aider: nil session")
	}

	var errs []error

	// 1. Graceful exit.
	if err := r.tmuxClient.SendKeys(session.TmuxSession, session.Window, session.Pane, "/exit Enter"); err != nil {
		errs = append(errs, fmt.Errorf("sending /exit: %w", err))
	}

	// 2. Let aider actually terminate before we yank the pane.
	time.Sleep(2 * time.Second)

	// 3. Kill tmux session regardless of whether /exit went through.
	if err := r.tmuxClient.KillSession(session.TmuxSession); err != nil {
		errs = append(errs, fmt.Errorf("killing session %q: %w", session.TmuxSession, err))
	}

	// 4. Best-effort temp-file cleanup. We do NOT fail Cleanup over a missing
	// or unremovable temp file — log via error, but don't promote it over
	// session-teardown errors.
	if session.Metadata != nil {
		if path := session.Metadata["systemPromptFile"]; path != "" {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				errs = append(errs, fmt.Errorf("removing prompt file %q: %w", path, err))
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

// -------------------------------------------------------------------------
// Unexported helpers
// -------------------------------------------------------------------------

// aiderTokensRe matches aider's usage line. Group 1 is sent tokens, group 2
// is received tokens, optional group 3 is the dollar cost.
//
// Example matches:
//
//	"Tokens: 1,234 sent, 567 received, $0.01 cost"
//	"Tokens: 500 sent, 200 received"
var aiderTokensRe = regexp.MustCompile(
	`(?i)tokens:\s*([\d,]+)\s*sent,\s*([\d,]+)\s*received(?:,\s*\$([0-9]+\.?[0-9]*)\s*cost)?`,
)

// buildCLICommand produces the full "aider <flags>" invocation.
//
// Always-on flags:
//
//	--no-auto-commits  gitops owns git; aider must not race the supervisor.
//	--yes              worker runs non-interactively; auto-accept prompts.
//
// Conditional flags:
//
//	--model <m>                    when cfg.Model set
//	--message-file <systemPrompt>  when systemPromptFile non-empty
//	<ExtraCLIArgs>                 verbatim tail, caller-controlled
//
// Notes on fields intentionally IGNORED (aider has no counterpart):
//   - AllowedTools, DisallowedTools
//   - PermissionMode
func buildCLICommand(cfg agent.SpawnConfig, systemPromptFile string) string {
	parts := []string{"aider"}

	if cfg.Model != "" {
		parts = append(parts, "--model", cfg.Model)
	}

	// Always-on safety flags.
	parts = append(parts, "--no-auto-commits", "--yes")

	if systemPromptFile != "" {
		parts = append(parts, "--message-file", runtimeutil.ShellEscape(systemPromptFile))
	}

	// cfg.ExtraCLIArgs is a trusted config value (sourced from SkillProfile /
	// tier YAML, never from user input). It is appended verbatim without
	// shell escaping so callers can pass pre-formed flag strings.
	if cfg.ExtraCLIArgs != "" {
		parts = append(parts, cfg.ExtraCLIArgs)
	}

	return strings.Join(parts, " ")
}

// isAiderReady reports whether the captured pane content indicates the aider
// CLI is ready for input. Returns true if any trimmed line is:
//   - case-insensitive equal to "aider>" or starts with "aider> "
//   - equal to ">" or starts with "> " (fallback; aider sometimes strips the
//     "aider" prefix depending on pane width / ANSI handling)
func isAiderReady(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if lower == "aider>" || strings.HasPrefix(lower, "aider> ") {
			return true
		}
		if trimmed == ">" || strings.HasPrefix(trimmed, "> ") {
			return true
		}
	}
	return false
}

// isAiderIdle reports whether the LAST non-empty line of content is a bare
// aider idle prompt. Accepts ">", "> ", "aider>", "aider> " (case-insensitive
// for the "aider" variant). Requiring the prompt to be the terminal line
// avoids false positives when ">" appears earlier in the buffer followed by
// user-entered text.
func isAiderIdle(content string) bool {
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		return trimmed == ">" || trimmed == "> " || lower == "aider>" || lower == "aider> "
	}
	return false
}

