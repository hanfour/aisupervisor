// Package aisagent implements the agent.AgentRuntime interface for the
// `ais-agent` CLI — the multi-provider runtime used by employee-growth workers.
//
// The helpers in this file (buildCLICommand, isAISAgentReady, isAISAgentIdle,
// shellEscape, token-parsing regexes, promptRenderDelay) intentionally mirror
// the sibling claudecode package. Duplication is expected for Phase 3 Task 3;
// Task 5 will revisit consolidation once the spawner has been refactored to
// consume runtimes instead of open-coding these details.
//
// Conventions that differ from Claude Code:
//
//   - ais-agent does NOT have a --dangerously-skip-permissions flag. All
//     permission handling goes through --permission-mode <mode>.
//   - --allowed-tools takes a SINGLE comma-separated argument, not a
//     sequence of shell-escaped tokens.
//   - ais-agent v0.1.0 does NOT support --disallowed-tools. SpawnConfig.
//     DisallowedTools is silently dropped when this runtime is selected.
//     Spawner-side autonomous skill isolation is the load-bearing barrier
//     for tool restriction; ais-agent additionally does not load .claude/
//     skills, so the SessionStart-hook isolation that motivates Claude
//     Code's DisallowedTools list does not apply here.
//   - Provider selection (--provider) and max-token budget (--max-tokens) are
//     not first-class fields on agent.SpawnConfig. To keep the shared struct
//     backend-agnostic, this runtime reads them from SpawnConfig.EnvVars:
//     - EnvVars["AIS_PROVIDER"]   → --provider <value>    (if non-empty)
//     - EnvVars["AIS_MAX_TOKENS"] → --max-tokens <int>    (if parse > 0)
//     Callers that want these flags should populate EnvVars accordingly.
package aisagent

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

// Runtime drives the ais-agent CLI inside a tmux pane.
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
func (r *Runtime) Name() string { return "ais-agent" }

// supportedProviders is the set of LLM provider names ais-agent v0.1.0
// accepts via --provider. The bundled binary exits at startup with
// "Failed to create provider: unknown provider: <name>" when given any
// other value (notably "ollama"), and the absence of a banner means
// DetectReady would otherwise burn the full ~120s timeout before the
// spawner's claude fallback fires. Listed in lookup order; extend this
// when bumping the ais-agent dependency. Empty string ("") means
// "no --provider flag will be passed" and is always allowed.
var supportedProviders = map[string]struct{}{
	"":          {}, // no --provider passed
	"openai":    {},
	"anthropic": {},
}

// Validate rejects SpawnConfigs whose AIS_PROVIDER env var names a
// provider this ais-agent build cannot drive. Returning a fast error
// here (instead of at DetectReady) lets the spawner skip straight to
// its claude fallback in <1s rather than waiting out the 120s
// DetectReady timeout for a CLI that has already exited.
func (r *Runtime) Validate(cfg agent.SpawnConfig) error {
	provider := cfg.EnvVars["AIS_PROVIDER"]
	if _, ok := supportedProviders[provider]; !ok {
		supported := []string{"(empty)", "openai", "anthropic"}
		return fmt.Errorf("aisagent: unsupported provider %q (supported: %s)", provider, strings.Join(supported, ", "))
	}
	return nil
}

// MonitoredSessionType returns the tool-type tag used by supervisor.MonitoredSession.
func (r *Runtime) MonitoredSessionType() string { return "ais_agent" }

// Spawn creates a new tmux session, primes the shell, and launches the
// ais-agent CLI using flags derived from cfg. Unlike the Claude Code runtime
// this does NOT unset CLAUDECODE because ais-agent has no nested-session
// guard. Callers must still follow up with DetectReady before sending prompts
// via SendPrompt.
func (r *Runtime) Spawn(ctx context.Context, cfg agent.SpawnConfig) (*agent.AgentSession, error) {
	if r.tmuxClient == nil {
		return nil, fmt.Errorf("aisagent: tmux client is nil")
	}

	name, err := runtimeutil.NewSessionName("ais")
	if err != nil {
		return nil, fmt.Errorf("generating session name: %w", err)
	}

	if err := r.tmuxClient.CreateSession(name); err != nil {
		return nil, fmt.Errorf("creating tmux session %q: %w", name, err)
	}

	// Best-effort teardown if any subsequent setup step fails.
	cleanup := func(reason error) (*agent.AgentSession, error) {
		if killErr := r.tmuxClient.KillSession(name); killErr != nil {
			return nil, fmt.Errorf("%w (also failed to kill session: %v)", reason, killErr)
		}
		return nil, reason
	}

	// 1. cd into the working directory so the CLI inherits it.
	if cfg.WorkDir != "" {
		if err := r.tmuxClient.SendKeys(name, 0, 0, fmt.Sprintf("cd %s", runtimeutil.ShellEscape(cfg.WorkDir))+" Enter"); err != nil {
			return cleanup(fmt.Errorf("sending cd: %w", err))
		}
		time.Sleep(300 * time.Millisecond)
	}

	// 2. Launch ais-agent with the composed flags. No CLAUDECODE unset step —
	// ais-agent does not care about that environment variable.
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
		return fmt.Errorf("aisagent: tmux client is nil")
	}
	if session == nil {
		return fmt.Errorf("aisagent: nil session")
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
		return "", fmt.Errorf("aisagent: tmux client is nil")
	}
	if session == nil {
		return "", fmt.Errorf("aisagent: nil session")
	}
	return r.tmuxClient.CapturePane(session.TmuxSession, session.Window, session.Pane, lines)
}

// DetectReady polls the pane until isAISAgentReady returns true or the timeout
// elapses. Unlike the Claude Code runtime there are no trust-folder or
// skip-permissions dialogs to auto-accept — ais-agent does not prompt for
// permissions interactively.
func (r *Runtime) DetectReady(ctx context.Context, session *agent.AgentSession, timeout time.Duration) error {
	if r.tmuxClient == nil {
		return fmt.Errorf("aisagent: tmux client is nil")
	}
	if session == nil {
		return fmt.Errorf("aisagent: nil session")
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
			return fmt.Errorf("aisagent: timeout waiting for CLI ready after %d polls", pollCount)
		case <-ticker.C:
			pollCount++
			content, err := r.tmuxClient.CapturePane(session.TmuxSession, session.Window, session.Pane, 10)
			if err != nil {
				continue
			}
			if isAISAgentReady(content) {
				return nil
			}
		}
	}
}

// DetectCompletion is a pure check over already-captured pane content:
// returns true iff the last non-empty line is a bare ais-agent idle prompt.
// Callers own the pane-capture cadence.
func (r *Runtime) DetectCompletion(ctx context.Context, session *agent.AgentSession, content string) (bool, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return false, err
		}
	}
	return isAISAgentIdle(content), nil
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
		return fmt.Errorf("aisagent: tmux client is nil")
	}
	if session == nil {
		return fmt.Errorf("aisagent: nil session")
	}
	return r.tmuxClient.KillSession(session.TmuxSession)
}

// -------------------------------------------------------------------------
// Unexported helpers
// -------------------------------------------------------------------------

// buildCLICommand produces the full "ais-agent <flags>" invocation.
//
// Flag order, chosen to read top-to-bottom from most important to least:
//  1. --provider          (from EnvVars["AIS_PROVIDER"])
//  2. --model
//  3. --permission-mode   (ais-agent has no --dangerously-skip-permissions)
//  4. --allowed-tools <csv>
//  5. --append-system-prompt <escaped>
//  6. --max-tokens        (from EnvVars["AIS_MAX_TOKENS"])
//  7. ExtraCLIArgs (verbatim, caller-controlled)
//
// Note: SpawnConfig.DisallowedTools is intentionally NOT forwarded.
// ais-agent v0.1.0 does not implement a --disallowed-tools flag, so
// passing one causes the CLI to exit immediately with
// "flag provided but not defined: -disallowed-tools" — which previously
// cost every spawn ~120s waiting for ais-agent's DetectReady to time
// out before falling back to claude.
func buildCLICommand(cfg agent.SpawnConfig) string {
	parts := []string{"ais-agent"}

	if provider := cfg.EnvVars["AIS_PROVIDER"]; provider != "" {
		parts = append(parts, "--provider", provider)
	}

	if cfg.Model != "" {
		parts = append(parts, "--model", cfg.Model)
	}

	if cfg.PermissionMode != "" {
		// ais-agent has no --dangerously-skip-permissions counterpart.
		// Forward the mode string verbatim — the CLI validates it.
		parts = append(parts, "--permission-mode", cfg.PermissionMode)
	}

	if len(cfg.AllowedTools) > 0 {
		parts = append(parts, "--allowed-tools", strings.Join(cfg.AllowedTools, ","))
	}

	// cfg.DisallowedTools is intentionally dropped — see func comment.

	if cfg.SystemPrompt != "" {
		parts = append(parts, "--append-system-prompt", runtimeutil.ShellEscape(cfg.SystemPrompt))
	}

	if raw := cfg.EnvVars["AIS_MAX_TOKENS"]; raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			parts = append(parts, "--max-tokens", strconv.Itoa(n))
		}
	}

	// cfg.ExtraCLIArgs is a trusted config value (sourced from SkillProfile /
	// tier YAML, never from user input). It is appended verbatim without
	// shell escaping so callers can pass pre-formed flag strings.
	if cfg.ExtraCLIArgs != "" {
		parts = append(parts, cfg.ExtraCLIArgs)
	}

	return strings.Join(parts, " ")
}

// isAISAgentReady reports whether the captured pane content indicates the
// ais-agent CLI is ready for input. Returns true if:
//   - any trimmed line equals ">" or starts with "> "
//   - any trimmed line equals "ais>" or starts with "ais> "
//   - content contains "Ready for input", "Agent ready", or "ais-agent Ready"
//
// These banner patterns are documented here rather than read from a live
// ais-agent build because the project spec stipulates reasonable banner
// matching; tighten as the real CLI stabilises.
func isAISAgentReady(content string) bool {
	if strings.Contains(content, "Ready for input") ||
		strings.Contains(content, "Agent ready") ||
		strings.Contains(content, "ais-agent Ready") {
		return true
	}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == ">" || strings.HasPrefix(trimmed, "> ") ||
			trimmed == "ais>" || strings.HasPrefix(trimmed, "ais> ") {
			return true
		}
	}
	return false
}

// isAISAgentIdle reports whether the LAST non-empty line of content is a bare
// ais-agent idle prompt. This avoids false positives when the prompt appears
// earlier in the pane buffer followed by user-entered text.
func isAISAgentIdle(content string) bool {
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		return trimmed == ">" || trimmed == "> " || trimmed == "ais>" || trimmed == "ais> "
	}
	return false
}

