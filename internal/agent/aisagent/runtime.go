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
//   - --allowed-tools / --disallowed-tools take a SINGLE comma-separated
//     argument, not a sequence of shell-escaped tokens.
//   - ais-agent does not load .claude/ skills, so no SessionStart-hook skill
//     isolation is required. Callers may still supply DisallowedTools
//     explicitly; they will be forwarded.
//   - Provider selection (--provider) and max-token budget (--max-tokens) are
//     not first-class fields on agent.SpawnConfig. To keep the shared struct
//     backend-agnostic, this runtime reads them from SpawnConfig.EnvVars:
//     - EnvVars["AIS_PROVIDER"]   → --provider <value>    (if non-empty)
//     - EnvVars["AIS_MAX_TOKENS"] → --max-tokens <int>    (if parse > 0)
//     Callers that want these flags should populate EnvVars accordingly.
package aisagent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/agent"
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

// Spawn creates a new tmux session, primes the shell, and launches the
// ais-agent CLI using flags derived from cfg. Unlike the Claude Code runtime
// this does NOT unset CLAUDECODE because ais-agent has no nested-session
// guard. Callers must still follow up with DetectReady before sending prompts
// via SendPrompt.
func (r *Runtime) Spawn(ctx context.Context, cfg agent.SpawnConfig) (*agent.AgentSession, error) {
	if r.tmuxClient == nil {
		return nil, fmt.Errorf("aisagent: tmux client is nil")
	}

	name, err := newSessionName()
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
		if err := r.tmuxClient.SendKeys(name, 0, 0, fmt.Sprintf("cd %s", shellEscape(cfg.WorkDir))+" Enter"); err != nil {
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
	time.Sleep(promptRenderDelay(len(prompt)))
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

// DetectCompletion performs a single pane capture and reports whether the CLI
// is back at an idle prompt. Callers are expected to poll this method on
// whatever cadence suits them.
func (r *Runtime) DetectCompletion(ctx context.Context, session *agent.AgentSession) (bool, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return false, err
		}
	}
	if r.tmuxClient == nil {
		return false, fmt.Errorf("aisagent: tmux client is nil")
	}
	if session == nil {
		return false, fmt.Errorf("aisagent: nil session")
	}
	content, err := r.tmuxClient.CapturePane(session.TmuxSession, session.Window, session.Pane, 20)
	if err != nil {
		return false, fmt.Errorf("capturing pane: %w", err)
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
	if matches := totalTokensRe.FindAllStringSubmatch(output, -1); len(matches) > 0 {
		last := matches[len(matches)-1]
		if val := parseTokenNum(last[1]); val > 0 {
			usage.InputTokens = val
		}
	}

	// Strategy 2: sum individual input/output token lines when no aggregate.
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		if ioMatches := ioTokensRe.FindAllStringSubmatch(output, -1); len(ioMatches) > 0 {
			for _, m := range ioMatches {
				val := parseTokenNum(m[2])
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
	if matches := costRe.FindAllStringSubmatch(output, -1); len(matches) > 0 {
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

// Token parsing regexes mirror the claudecode package (which in turn mirrors
// internal/worker/monitor.go). Copied rather than imported to keep the
// aisagent package free of dependencies on sibling runtimes or the (to-be-
// refactored) worker package.
var (
	totalTokensRe = regexp.MustCompile(`(?i)total\s+tokens?(?:\s+used)?[:\s]+([0-9][0-9,_]+)`)
	ioTokensRe    = regexp.MustCompile(`(?i)(input|output)\s+tokens?[:\s]+([0-9][0-9,_]+)`)
	costRe        = regexp.MustCompile(`(?i)total\s+cost[:\s]+\$([0-9]+\.?[0-9]*)`)
)

// parseTokenNum strips grouping separators and parses a base-10 integer.
func parseTokenNum(s string) int {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "_", "")
	val, _ := strconv.Atoi(s)
	return val
}

// buildCLICommand produces the full "ais-agent <flags>" invocation.
//
// Flag order, chosen to read top-to-bottom from most important to least:
//  1. --provider          (from EnvVars["AIS_PROVIDER"])
//  2. --model
//  3. --permission-mode   (ais-agent has no --dangerously-skip-permissions)
//  4. --allowed-tools <csv>
//  5. --disallowed-tools <csv>
//  6. --append-system-prompt <escaped>
//  7. --max-tokens        (from EnvVars["AIS_MAX_TOKENS"])
//  8. ExtraCLIArgs (verbatim, caller-controlled)
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

	if len(cfg.DisallowedTools) > 0 {
		parts = append(parts, "--disallowed-tools", strings.Join(cfg.DisallowedTools, ","))
	}

	if cfg.SystemPrompt != "" {
		parts = append(parts, "--append-system-prompt", shellEscape(cfg.SystemPrompt))
	}

	if raw := cfg.EnvVars["AIS_MAX_TOKENS"]; raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			parts = append(parts, "--max-tokens", strconv.Itoa(n))
		}
	}

	if cfg.ExtraCLIArgs != "" {
		parts = append(parts, cfg.ExtraCLIArgs)
	}

	return strings.Join(parts, " ")
}

// promptRenderDelay returns how long to wait after SendLiteralKeys before
// pressing Enter. Scales with prompt length: 1s base + 500ms per 2000 chars,
// capped at 5s. Matches internal/agent/claudecode and internal/worker/spawner.go.
func promptRenderDelay(promptLen int) time.Duration {
	base := 1 * time.Second
	extra := time.Duration(promptLen/2000) * 500 * time.Millisecond
	total := base + extra
	if total > 5*time.Second {
		total = 5 * time.Second
	}
	return total
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

// shellEscape wraps s in single quotes and escapes any embedded single quote
// by closing the quoted section, emitting a backslash-escaped quote, and
// reopening. Matches the helper used by internal/worker/spawner.go.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// newSessionName builds a unique tmux session identifier of the form
// "ais-<unix-nanos>-<hex4>". The random suffix guards against collisions when
// two spawns happen in the same nanosecond.
func newSessionName() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("ais-%d-%s", time.Now().UnixNano(), hex.EncodeToString(buf[:])), nil
}
