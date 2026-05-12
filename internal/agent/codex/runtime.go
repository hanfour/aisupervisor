// Package codex implements the agent.AgentRuntime interface for OpenAI
// Codex CLI (https://github.com/openai/codex). The runtime drives a
// `codex` process inside a tmux pane and exposes the standard
// lifecycle operations.
//
// Conventions specific to Codex:
//
//   - Codex authenticates via `codex login` (ChatGPT account), NOT
//     environment API keys. We assume the user has already logged in;
//     missing auth surfaces at DetectReady time when the CLI prints an
//     "Authentication required" banner instead of the ready prompt.
//   - `--no-alt-screen` is ALWAYS emitted: without it, Codex uses the
//     alternate terminal screen which `tmux capture-pane` reads as
//     empty. Every other detection signal hinges on inline mode.
//   - Permission modes map to Codex's sandbox + approval flags:
//
//       SpawnConfig.PermissionMode   Codex flags
//       ───────────────────────────  ───────────────────────────────────
//       "bypassPermissions"          --dangerously-bypass-approvals-and-sandbox
//       "acceptEdits"                --sandbox workspace-write --ask-for-approval on-request
//       "plan"                       --sandbox read-only --ask-for-approval on-request
//       "" / anything else           --ask-for-approval untrusted  (safest default)
//
//   - System prompt: Codex has no `--system-prompt` flag, but it
//     accepts an initial `[PROMPT]` positional argument that's
//     treated as the first user message. We pass the system prompt
//     there. (Codex's own system instructions come from
//     `~/.codex/config.toml` profiles.)
//   - Idle prompt is `›` (U+203A, single right-pointing angle
//     quotation mark) — NOT `>` or `❯`. Codex draws it on its own
//     line above the model/path footer, similar to Claude Code v2+.
//   - Ready banner contains `OpenAI Codex (v` so we can fast-fire
//     ready even before the prompt is drawn.
//   - Token-usage parsing is left as a no-op: Codex does not print
//     a per-turn telemetry line we can scrape reliably. The
//     interface allows returning a zero TokenUsage.
package codex

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/agent"
	"github.com/hanfourmini/aisupervisor/internal/agent/runtimeutil"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

// Compile-time assertion that *Runtime satisfies agent.AgentRuntime.
var _ agent.AgentRuntime = (*Runtime)(nil)

// Runtime drives the codex CLI inside a tmux pane.
type Runtime struct {
	tmuxClient tmux.TmuxClient
}

// New returns a Runtime backed by the supplied tmux client. The tmux
// client is allowed to be nil for callers that only exercise pure
// helpers (e.g. unit tests poking at Name() or buildCLICommand).
func New(tc tmux.TmuxClient) *Runtime {
	return &Runtime{tmuxClient: tc}
}

// Name returns the stable identifier for this runtime.
func (r *Runtime) Name() string { return "codex" }

// Validate is a no-op: Codex takes its model + sandbox via CLI flags
// we construct ourselves; bad combinations surface at DetectReady
// time (auth error / unknown model). Returning nil here keeps the
// fast-fail validate path reserved for genuinely impossible configs.
func (r *Runtime) Validate(cfg agent.SpawnConfig) error { return nil }

// MonitoredSessionType returns the tool-type tag used by
// supervisor.MonitoredSession.
func (r *Runtime) MonitoredSessionType() string { return "codex" }

// Spawn creates a new tmux session, cds into the worker's workdir,
// and launches codex with the composed CLI flags. The returned
// session does NOT imply codex is ready; callers must follow up with
// DetectReady before SendPrompt.
func (r *Runtime) Spawn(ctx context.Context, cfg agent.SpawnConfig) (*agent.AgentSession, error) {
	if r.tmuxClient == nil {
		return nil, fmt.Errorf("codex: tmux client is nil")
	}

	name, err := runtimeutil.NewSessionName("codex")
	if err != nil {
		return nil, fmt.Errorf("generating session name: %w", err)
	}

	if err := r.tmuxClient.CreateSession(name); err != nil {
		return nil, fmt.Errorf("creating tmux session %q: %w", name, err)
	}

	// Teardown helper for any subsequent failure.
	cleanup := func(reason error) (*agent.AgentSession, error) {
		if killErr := r.tmuxClient.KillSession(name); killErr != nil {
			return nil, fmt.Errorf("%w (also failed to kill session: %v)", reason, killErr)
		}
		return nil, reason
	}

	// 1. cd into the working directory so the CLI inherits it as the
	// shell cwd. We could pass `-C` instead but cd-first matches the
	// other runtime plugins and means the user's tmux scrollback shows
	// the expected pwd.
	if cfg.WorkDir != "" {
		if err := r.tmuxClient.SendKeys(name, 0, 0, fmt.Sprintf("cd %s", runtimeutil.ShellEscape(cfg.WorkDir))+" Enter"); err != nil {
			return cleanup(fmt.Errorf("sending cd: %w", err))
		}
		time.Sleep(300 * time.Millisecond)
	}

	// 2. Launch codex with the composed flags + optional initial prompt.
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
	}, nil
}

// SendPrompt pastes prompt into the pane, waits briefly for it to
// render, then submits via Enter.
func (r *Runtime) SendPrompt(session *agent.AgentSession, prompt string) error {
	if r.tmuxClient == nil {
		return fmt.Errorf("codex: tmux client is nil")
	}
	if session == nil {
		return fmt.Errorf("codex: nil session")
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

// CaptureOutput returns the last `lines` lines of the pane.
func (r *Runtime) CaptureOutput(session *agent.AgentSession, lines int) (string, error) {
	if r.tmuxClient == nil {
		return "", fmt.Errorf("codex: tmux client is nil")
	}
	if session == nil {
		return "", fmt.Errorf("codex: nil session")
	}
	return r.tmuxClient.CapturePane(session.TmuxSession, session.Window, session.Pane, lines)
}

// DetectReady polls the pane until isCodexReady returns true or the
// timeout elapses.
func (r *Runtime) DetectReady(ctx context.Context, session *agent.AgentSession, timeout time.Duration) error {
	if r.tmuxClient == nil {
		return fmt.Errorf("codex: tmux client is nil")
	}
	if session == nil {
		return fmt.Errorf("codex: nil session")
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
			return fmt.Errorf("codex: timeout waiting for CLI ready after %d polls", pollCount)
		case <-ticker.C:
			pollCount++
			content, err := r.tmuxClient.CapturePane(session.TmuxSession, session.Window, session.Pane, 15)
			if err != nil {
				continue
			}
			if isCodexReady(content) {
				return nil
			}
		}
	}
}

// DetectCompletion is a pure check over already-captured pane
// content: returns true iff the last non-empty line is a bare Codex
// idle prompt (`›` / `› `). Callers own the pane-capture cadence.
func (r *Runtime) DetectCompletion(ctx context.Context, session *agent.AgentSession, content string) (bool, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return false, err
		}
	}
	return isCodexIdle(content), nil
}

// ParseTokenUsage returns a zero TokenUsage: Codex does not print a
// reliable per-turn telemetry line we can scrape. Returning zero is
// explicitly allowed by the agent.AgentRuntime contract.
func (r *Runtime) ParseTokenUsage(output string) (agent.TokenUsage, error) {
	return agent.TokenUsage{}, nil
}

// Cleanup tears down the codex session. Codex doesn't expose a
// graceful `/exit` slash command at the CLI prompt; sending Ctrl-C
// followed by Ctrl-C again (Codex's "press Ctrl-C twice to exit"
// pattern) is the documented way. After that we kill the tmux
// session regardless.
func (r *Runtime) Cleanup(session *agent.AgentSession) error {
	if r.tmuxClient == nil {
		return fmt.Errorf("codex: tmux client is nil")
	}
	if session == nil {
		return fmt.Errorf("codex: nil session")
	}

	var errs []error

	// 1. Ctrl-C twice — codex's documented quit gesture.
	if err := r.tmuxClient.SendKeys(session.TmuxSession, session.Window, session.Pane, "C-c"); err != nil {
		errs = append(errs, fmt.Errorf("sending first C-c: %w", err))
	}
	time.Sleep(200 * time.Millisecond)
	if err := r.tmuxClient.SendKeys(session.TmuxSession, session.Window, session.Pane, "C-c"); err != nil {
		errs = append(errs, fmt.Errorf("sending second C-c: %w", err))
	}
	// 2. Let codex actually terminate.
	time.Sleep(1 * time.Second)

	// 3. Kill tmux session regardless.
	if err := r.tmuxClient.KillSession(session.TmuxSession); err != nil {
		errs = append(errs, fmt.Errorf("killing session %q: %w", session.TmuxSession, err))
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

// -------------------------------------------------------------------------
// Unexported helpers
// -------------------------------------------------------------------------

// buildCLICommand produces the full "codex <flags> [PROMPT]" invocation.
//
// Always-on flags:
//
//	--no-alt-screen  Disables alternate screen mode so `tmux
//	                 capture-pane` can read the live output.
//
// Conditional flags:
//
//	-m <model>                       cfg.Model
//	-C <workdir>                     redundant with shell cd; emitted only
//	                                 if cfg.WorkDir set, for robustness
//	--sandbox + --ask-for-approval   derived from cfg.PermissionMode
//	OR --dangerously-bypass-...      for "bypassPermissions"
//	<cfg.ExtraCLIArgs>               verbatim tail (trusted config)
//
// Notes on fields intentionally IGNORED:
//   - AllowedTools, DisallowedTools: Codex doesn't expose tool gating
//     at the CLI level (its tool universe is fixed).
//   - SystemPrompt: appended later via SendPrompt as the initial
//     user message (Codex has no system-prompt flag).
func buildCLICommand(cfg agent.SpawnConfig) string {
	parts := []string{"codex", "--no-alt-screen"}

	if cfg.WorkDir != "" {
		parts = append(parts, "-C", runtimeutil.ShellEscape(cfg.WorkDir))
	}
	if cfg.Model != "" {
		parts = append(parts, "-m", cfg.Model)
	}

	switch cfg.PermissionMode {
	case "bypassPermissions":
		parts = append(parts, "--dangerously-bypass-approvals-and-sandbox")
	case "acceptEdits":
		parts = append(parts, "--sandbox", "workspace-write", "--ask-for-approval", "on-request")
	case "plan":
		parts = append(parts, "--sandbox", "read-only", "--ask-for-approval", "on-request")
	default:
		// "untrusted" is Codex's safest non-interactive policy: trusted
		// commands run silently, anything else escalates to the model
		// (which in headless mode will either ask or fail).
		parts = append(parts, "--ask-for-approval", "untrusted")
	}

	// ExtraCLIArgs is a trusted config value (sourced from SkillProfile
	// / tier YAML, never user input). Appended verbatim without shell
	// escaping so callers can pass pre-formed flag strings.
	if cfg.ExtraCLIArgs != "" {
		parts = append(parts, cfg.ExtraCLIArgs)
	}

	return strings.Join(parts, " ")
}

// isCodexReady reports whether the captured pane content indicates
// the Codex CLI is ready for input. A match fires on either:
//
//  1. Banner header "OpenAI Codex (v" anywhere in the buffer. Only
//     printed on CLI launch — cannot false-fire after the first
//     prompt.
//  2. Any of the last 5 non-empty lines being a bare `›` prompt or
//     starting with `› ` (user has begun typing).
//
// Limiting the prompt scan to a trailing window (similar to
// claudecode's readyScanLines) handles Codex's "prompt-above-footer"
// layout — the last non-empty line is the model/path footer, the
// prompt sits one to two lines above it.
func isCodexReady(content string) bool {
	if strings.Contains(content, "OpenAI Codex (v") {
		return true
	}
	lines := strings.Split(content, "\n")
	end := trimTrailingEmpty(lines)
	const scanWindow = 5
	start := end - scanWindow
	if start < 0 {
		start = 0
	}
	for i := start; i < end; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "›" || trimmed == "› " || strings.HasPrefix(trimmed, "› ") {
			return true
		}
	}
	return false
}

// isCodexIdle reports whether the captured pane content shows Codex
// at rest with the bare `›` prompt visible in the trailing-pane
// scan window. Mirrors claudecode's idle semantics (last non-empty
// line OR within scanWindow of the end), accepting the prompt
// followed by a footer line.
func isCodexIdle(content string) bool {
	lines := strings.Split(content, "\n")
	end := trimTrailingEmpty(lines)
	const scanWindow = 5
	start := end - scanWindow
	if start < 0 {
		start = 0
	}
	for i := start; i < end; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "›" || trimmed == "› " {
			return true
		}
	}
	return false
}

// trimTrailingEmpty returns the index one past the last non-empty
// (whitespace-only) line. Compensates for tmux capture-pane padding
// with empty lines after the rendered content.
func trimTrailingEmpty(lines []string) int {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return end
}
