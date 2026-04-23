package aider

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/agent"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

// TestAiderRuntime_Name verifies the runtime self-identifies as "aider".
func TestAiderRuntime_Name(t *testing.T) {
	r := New(nil)
	if got := r.Name(); got != "aider" {
		t.Fatalf("Name() = %q, want %q", got, "aider")
	}
}

// TestAiderRuntime_InterfaceSatisfied ensures *Runtime implements agent.AgentRuntime.
func TestAiderRuntime_InterfaceSatisfied(t *testing.T) {
	var _ agent.AgentRuntime = (*Runtime)(nil)
}

// TestAiderRuntime_BuildCLICommand verifies that a populated SpawnConfig is
// translated into the expected CLI flags for the aider binary.
//
// Aider flag conventions (differ significantly from Claude Code / ais-agent):
//   - Binary is "aider"
//   - --model <m> when Model set
//   - --no-auto-commits is ALWAYS emitted (gitops owns git)
//   - --yes is ALWAYS emitted (auto-confirm prompts)
//   - Aider has NO --allowed-tools, --disallowed-tools, --permission-mode,
//     or --dangerously-skip-permissions flags. These MUST NOT be emitted.
//   - ExtraCLIArgs appended verbatim at the tail.
func TestAiderRuntime_BuildCLICommand(t *testing.T) {
	cfg := agent.SpawnConfig{
		Model:           "gpt-4o",
		PermissionMode:  "acceptEdits",
		SystemPrompt:    "You are a helpful assistant.",
		AllowedTools:    []string{"Bash", "Edit", "Read"},
		DisallowedTools: []string{"Skill", "EnterPlanMode"},
		ExtraCLIArgs:    "--verbose",
	}

	cmd := buildCLICommand(cfg, "")

	// Must start with the aider binary.
	if !strings.HasPrefix(cmd, "aider ") {
		t.Fatalf("buildCLICommand: expected binary prefix %q, got %q", "aider", cmd)
	}

	// --model <m>
	if !strings.Contains(cmd, "--model gpt-4o") {
		t.Errorf("buildCLICommand: missing --model flag, got %q", cmd)
	}

	// --no-auto-commits ALWAYS
	if !strings.Contains(cmd, "--no-auto-commits") {
		t.Errorf("buildCLICommand: missing --no-auto-commits, got %q", cmd)
	}

	// --yes ALWAYS
	if !strings.Contains(cmd, "--yes") {
		t.Errorf("buildCLICommand: missing --yes, got %q", cmd)
	}

	// Aider has NO tool/permission flags. Explicitly check they are omitted.
	forbidden := []string{
		"--allowed-tools",
		"--allowedTools",
		"--disallowed-tools",
		"--disallowedTools",
		"--permission-mode",
		"--dangerously-skip-permissions",
	}
	for _, flag := range forbidden {
		if strings.Contains(cmd, flag) {
			t.Errorf("buildCLICommand: unexpected %s in aider command: %q", flag, cmd)
		}
	}

	// ExtraCLIArgs appended verbatim at the end.
	if !strings.HasSuffix(cmd, "--verbose") {
		t.Errorf("buildCLICommand: expected ExtraCLIArgs trailing the command, got %q", cmd)
	}
}

// TestAiderRuntime_BuildCLICommand_EmptyConfig verifies the no-op path still
// includes the two always-on safety flags.
func TestAiderRuntime_BuildCLICommand_EmptyConfig(t *testing.T) {
	cmd := buildCLICommand(agent.SpawnConfig{}, "")
	if !strings.HasPrefix(cmd, "aider ") && cmd != "aider" {
		t.Errorf("empty cfg: expected aider binary prefix, got %q", cmd)
	}
	if !strings.Contains(cmd, "--no-auto-commits") {
		t.Errorf("empty cfg: expected --no-auto-commits, got %q", cmd)
	}
	if !strings.Contains(cmd, "--yes") {
		t.Errorf("empty cfg: expected --yes, got %q", cmd)
	}
}

// TestAiderRuntime_BuildCLICommand_MessageFile verifies that a non-empty
// systemPromptFile path is forwarded as --message-file <path>.
func TestAiderRuntime_BuildCLICommand_MessageFile(t *testing.T) {
	cfg := agent.SpawnConfig{Model: "gpt-4o"}
	cmd := buildCLICommand(cfg, "/tmp/aider-prompt-abc123.md")
	if !strings.Contains(cmd, "--message-file /tmp/aider-prompt-abc123.md") {
		t.Errorf("buildCLICommand: expected --message-file flag, got %q", cmd)
	}
}

// TestAiderRuntime_ParseReadyIndicators verifies isAiderReady matches documented
// ready patterns and rejects random content or in-progress output.
func TestAiderRuntime_ParseReadyIndicators(t *testing.T) {
	readyCases := []struct {
		name    string
		content string
	}{
		{"bare aider prompt", "aider>"},
		{"aider prompt with space", "aider> "},
		{"aider prompt uppercase", "AIDER>"},
		{"aider prompt mixed case", "Aider>"},
		{"aider prompt with trailing text", "aider> start typing\n"},
		{"aider prompt on its own line", "starting up\naider>\n"},
		{"bare gt prompt (fallback)", ">"},
		{"gt prompt with space", "> "},
		{"gt prompt on its own line", "initializing...\n>\n"},
		{"gt prompt with trailing text", "> type here\n"},
	}
	for _, tc := range readyCases {
		t.Run("ready/"+tc.name, func(t *testing.T) {
			if !isAiderReady(tc.content) {
				t.Errorf("isAiderReady(%q) = false, want true", tc.content)
			}
		})
	}

	notReadyCases := []struct {
		name    string
		content string
	}{
		{"empty", ""},
		{"random noise", "loading modules...\nindexing files..."},
		{"progress spinner", "⠋ Thinking...\n⠙ Thinking..."},
		{"shell only", "user@host:~$"},
		{"unrelated word", "already running"},
	}
	for _, tc := range notReadyCases {
		t.Run("notready/"+tc.name, func(t *testing.T) {
			if isAiderReady(tc.content) {
				t.Errorf("isAiderReady(%q) = true, want false", tc.content)
			}
		})
	}
}

// TestAiderRuntime_ParseTokenUsage verifies the aider-specific telemetry line
// parser. Aider emits lines like:
//
//	"Tokens: 1,234 sent, 567 received, $0.01 cost"
//
// or without cost:
//
//	"Tokens: 1,234 sent, 567 received"
//
// Missing pattern returns zero-value + nil error.
func TestAiderRuntime_ParseTokenUsage(t *testing.T) {
	r := New(nil)

	t.Run("tokens and cost", func(t *testing.T) {
		content := "Tokens: 1,234 sent, 567 received, $0.01 cost"
		usage, err := r.ParseTokenUsage(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if usage.InputTokens != 1234 {
			t.Errorf("InputTokens = %d, want 1234", usage.InputTokens)
		}
		if usage.OutputTokens != 567 {
			t.Errorf("OutputTokens = %d, want 567", usage.OutputTokens)
		}
		if usage.TotalCost != 0.01 {
			t.Errorf("TotalCost = %v, want 0.01", usage.TotalCost)
		}
	})

	t.Run("tokens without cost", func(t *testing.T) {
		content := "Tokens: 500 sent, 200 received"
		usage, err := r.ParseTokenUsage(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if usage.InputTokens != 500 {
			t.Errorf("InputTokens = %d, want 500", usage.InputTokens)
		}
		if usage.OutputTokens != 200 {
			t.Errorf("OutputTokens = %d, want 200", usage.OutputTokens)
		}
		if usage.TotalCost != 0.0 {
			t.Errorf("TotalCost = %v, want 0.0", usage.TotalCost)
		}
	})

	t.Run("case-insensitive", func(t *testing.T) {
		content := "TOKENS: 100 SENT, 50 RECEIVED, $0.002 COST"
		usage, err := r.ParseTokenUsage(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if usage.InputTokens != 100 || usage.OutputTokens != 50 {
			t.Errorf("case-insensitive failed: got %+v", usage)
		}
		if usage.TotalCost != 0.002 {
			t.Errorf("TotalCost = %v, want 0.002", usage.TotalCost)
		}
	})

	t.Run("missing pattern returns zero value", func(t *testing.T) {
		usage, err := r.ParseTokenUsage("no usage metrics here")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if (usage != agent.TokenUsage{}) {
			t.Fatalf("expected zero value TokenUsage, got %+v", usage)
		}
	})

	t.Run("embedded in larger output", func(t *testing.T) {
		content := "some preamble text\nTokens: 9,999 sent, 1,111 received, $0.25 cost\ntrailing noise\n"
		usage, err := r.ParseTokenUsage(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if usage.InputTokens != 9999 {
			t.Errorf("InputTokens = %d, want 9999", usage.InputTokens)
		}
		if usage.OutputTokens != 1111 {
			t.Errorf("OutputTokens = %d, want 1111", usage.OutputTokens)
		}
		if usage.TotalCost != 0.25 {
			t.Errorf("TotalCost = %v, want 0.25", usage.TotalCost)
		}
	})
}

// -----------------------------------------------------------------------------
// Recording mock TmuxClient for Cleanup test.
// -----------------------------------------------------------------------------

type tmuxCall struct {
	method  string
	session string
	keys    string // for SendKeys / SendLiteralKeys
}

type recordingTmux struct {
	mu    sync.Mutex
	calls []tmuxCall
}

func (m *recordingTmux) record(c tmuxCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, c)
}

func (m *recordingTmux) snapshot() []tmuxCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]tmuxCall, len(m.calls))
	copy(out, m.calls)
	return out
}

func (m *recordingTmux) ListSessions() ([]tmux.SessionInfo, error) { return nil, nil }
func (m *recordingTmux) ListPanes(session string) ([]tmux.PaneInfo, error) {
	return nil, nil
}
func (m *recordingTmux) CapturePane(session string, window, pane, lines int) (string, error) {
	return "", nil
}
func (m *recordingTmux) SendKeys(session string, window, pane int, keys string) error {
	m.record(tmuxCall{method: "SendKeys", session: session, keys: keys})
	return nil
}
func (m *recordingTmux) SendLiteralKeys(session string, window, pane int, text string) error {
	m.record(tmuxCall{method: "SendLiteralKeys", session: session, keys: text})
	return nil
}
func (m *recordingTmux) CreateSession(name string) error { return nil }
func (m *recordingTmux) KillSession(name string) error {
	m.record(tmuxCall{method: "KillSession", session: name})
	return nil
}
func (m *recordingTmux) HasSession(name string) (bool, error) { return false, nil }

// TestAiderRuntime_CleanupSendsExit verifies:
//  1. SendKeys with /exit (and Enter) is emitted before KillSession.
//  2. KillSession is called on the correct session name.
//  3. There is a sleep between the two (acceptable test runtime: ~2s).
func TestAiderRuntime_CleanupSendsExit(t *testing.T) {
	mock := &recordingTmux{}
	r := New(mock)
	sess := &agent.AgentSession{
		RuntimeName: "aider",
		TmuxSession: "aider-test-session",
		Window:      0,
		Pane:        0,
		Metadata:    map[string]string{},
	}

	start := time.Now()
	if err := r.Cleanup(sess); err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	elapsed := time.Since(start)

	calls := mock.snapshot()
	if len(calls) < 2 {
		t.Fatalf("expected at least 2 calls (SendKeys + KillSession), got %d: %+v", len(calls), calls)
	}

	// Find the /exit SendKeys and the KillSession call.
	var exitIdx, killIdx = -1, -1
	for i, c := range calls {
		if c.method == "SendKeys" && strings.Contains(c.keys, "/exit") {
			exitIdx = i
		}
		if c.method == "KillSession" {
			killIdx = i
		}
	}
	if exitIdx == -1 {
		t.Fatalf("expected a SendKeys call containing /exit, got calls: %+v", calls)
	}
	if killIdx == -1 {
		t.Fatalf("expected a KillSession call, got calls: %+v", calls)
	}
	if exitIdx >= killIdx {
		t.Fatalf("expected /exit SendKeys BEFORE KillSession; exitIdx=%d killIdx=%d calls=%+v",
			exitIdx, killIdx, calls)
	}

	// Session-name plumbed correctly.
	if calls[exitIdx].session != "aider-test-session" {
		t.Errorf("SendKeys session = %q, want %q", calls[exitIdx].session, "aider-test-session")
	}
	if calls[killIdx].session != "aider-test-session" {
		t.Errorf("KillSession session = %q, want %q", calls[killIdx].session, "aider-test-session")
	}

	// The sleep between /exit and KillSession is ~2s. We do not assert an exact
	// duration, but the total Cleanup runtime must exceed 1s (i.e. the sleep
	// actually happened). Upper-bounded loosely at 5s to catch gross regressions.
	if elapsed < 1*time.Second {
		t.Errorf("Cleanup finished in %v — expected >=1s to allow /exit to flush", elapsed)
	}
	if elapsed > 5*time.Second {
		t.Errorf("Cleanup took %v — expected <=5s (2s sleep + slack)", elapsed)
	}
}

// TestAiderRuntime_PromptRenderDelay verifies the delay-scaling formula matches
// the siblings (claudecode, aisagent).
func TestAiderRuntime_PromptRenderDelay(t *testing.T) {
	tests := []struct {
		promptLen int
		want      time.Duration
	}{
		{0, 1 * time.Second},
		{1999, 1 * time.Second},
		{2000, 1500 * time.Millisecond},
		{4000, 2 * time.Second},
		{100000, 5 * time.Second},
	}
	for _, tc := range tests {
		if got := promptRenderDelay(tc.promptLen); got != tc.want {
			t.Errorf("promptRenderDelay(%d) = %v, want %v", tc.promptLen, got, tc.want)
		}
	}
}

// TestAiderRuntime_ShellEscape verifies the shellEscape helper matches the
// sibling convention (single-quote wrap, escape embedded quotes).
func TestAiderRuntime_ShellEscape(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"simple", "'simple'"},
		{"with spaces", "'with spaces'"},
		{"don't", `'don'\''t'`},
		{"", "''"},
	}
	for _, tc := range tests {
		if got := shellEscape(tc.in); got != tc.want {
			t.Errorf("shellEscape(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
