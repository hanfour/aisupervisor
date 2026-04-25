package claudecode

import (
	"strings"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/agent"
)

// TestClaudeCodeRuntime_Name verifies the runtime self-identifies as "claude".
func TestClaudeCodeRuntime_Name(t *testing.T) {
	r := New(nil)
	if got := r.Name(); got != "claude" {
		t.Fatalf("Name() = %q, want %q", got, "claude")
	}
}

// TestClaudeCodeRuntime_InterfaceSatisfied ensures *Runtime implements agent.AgentRuntime.
func TestClaudeCodeRuntime_InterfaceSatisfied(t *testing.T) {
	var _ agent.AgentRuntime = (*Runtime)(nil)
}

// TestClaudeCodeRuntime_BuildCLICommand verifies that all SpawnConfig fields are
// translated into the expected CLI flags in the expected order.
func TestClaudeCodeRuntime_BuildCLICommand(t *testing.T) {
	cfg := agent.SpawnConfig{
		Model:           "claude-sonnet-4-5",
		PermissionMode:  "acceptEdits",
		SystemPrompt:    "You are a helpful assistant.",
		AllowedTools:    []string{"Bash", "Edit", "Read"},
		DisallowedTools: []string{"Skill", "EnterPlanMode"},
		ExtraCLIArgs:    "--verbose",
	}

	cmd := buildCLICommand(cfg)

	// Must start with the claude binary.
	if !strings.HasPrefix(cmd, "claude ") {
		t.Fatalf("buildCLICommand: expected binary prefix %q, got %q", "claude", cmd)
	}

	// --model <m>
	if !strings.Contains(cmd, "--model claude-sonnet-4-5") {
		t.Errorf("buildCLICommand: missing --model flag, got %q", cmd)
	}

	// --permission-mode <mode> (acceptEdits is NOT bypassPermissions)
	if !strings.Contains(cmd, "--permission-mode acceptEdits") {
		t.Errorf("buildCLICommand: missing --permission-mode flag, got %q", cmd)
	}
	if strings.Contains(cmd, "--dangerously-skip-permissions") {
		t.Errorf("buildCLICommand: unexpected --dangerously-skip-permissions for non-bypass mode, got %q", cmd)
	}

	// --allowedTools followed by each tool as a separate shell-escaped token.
	if !strings.Contains(cmd, "--allowedTools 'Bash' 'Edit' 'Read'") {
		t.Errorf("buildCLICommand: expected escaped allowedTools tokens, got %q", cmd)
	}

	// --disallowedTools
	if !strings.Contains(cmd, "--disallowedTools 'Skill' 'EnterPlanMode'") {
		t.Errorf("buildCLICommand: expected escaped disallowedTools tokens, got %q", cmd)
	}

	// --append-system-prompt <shell-escaped system prompt>
	if !strings.Contains(cmd, "--append-system-prompt 'You are a helpful assistant.'") {
		t.Errorf("buildCLICommand: expected shell-escaped system prompt, got %q", cmd)
	}

	// ExtraCLIArgs appended verbatim at the end.
	if !strings.HasSuffix(cmd, "--verbose") {
		t.Errorf("buildCLICommand: expected ExtraCLIArgs trailing the command, got %q", cmd)
	}
}

// TestClaudeCodeRuntime_BuildCLICommand_BypassPermissions verifies that the
// bypassPermissions mode is mapped to --dangerously-skip-permissions (no mode flag).
func TestClaudeCodeRuntime_BuildCLICommand_BypassPermissions(t *testing.T) {
	cfg := agent.SpawnConfig{
		PermissionMode: "bypassPermissions",
	}
	cmd := buildCLICommand(cfg)

	if !strings.Contains(cmd, "--dangerously-skip-permissions") {
		t.Errorf("bypassPermissions: expected --dangerously-skip-permissions flag, got %q", cmd)
	}
	if strings.Contains(cmd, "--permission-mode") {
		t.Errorf("bypassPermissions: unexpected --permission-mode flag, got %q", cmd)
	}
}

// TestClaudeCodeRuntime_BuildCLICommand_EmptyConfig verifies that an empty
// SpawnConfig produces a bare "claude" command with no stray flags.
func TestClaudeCodeRuntime_BuildCLICommand_EmptyConfig(t *testing.T) {
	cmd := buildCLICommand(agent.SpawnConfig{})
	if strings.TrimSpace(cmd) != "claude" {
		t.Errorf("empty cfg: expected %q, got %q", "claude", cmd)
	}
}

// TestClaudeCodeRuntime_BuildCLICommand_SystemPromptEscaping verifies that a
// system prompt containing single quotes is correctly escaped.
func TestClaudeCodeRuntime_BuildCLICommand_SystemPromptEscaping(t *testing.T) {
	cfg := agent.SpawnConfig{
		SystemPrompt: "don't panic",
	}
	cmd := buildCLICommand(cfg)
	// shellEscape wraps in single quotes and replaces ' with '\''
	want := `--append-system-prompt 'don'\''t panic'`
	if !strings.Contains(cmd, want) {
		t.Errorf("escaping: expected %q in %q", want, cmd)
	}
}

// TestClaudeCodeRuntime_ParseReadyIndicators verifies isClaudeReady matches
// all documented ready patterns and rejects unrelated content.
func TestClaudeCodeRuntime_ParseReadyIndicators(t *testing.T) {
	readyCases := []struct {
		name    string
		content string
	}{
		{"bare gt prompt", ">"},
		{"gt prompt with space", "> "},
		{"bare caret prompt", "❯"},
		{"caret prompt with space", "❯ "},
		{"gt prompt on its own line", "initializing…\n>\n"},
		{"gt prompt with trailing text", "> what now\n"},
		{"caret prompt on its own line", "some setup log\n❯\n"},
		{"caret prompt with trailing text", "❯ type here\n"},
		{"what can i help banner", "Claude Code v2.0\nWhat can I help you with today?\n"},
		{"welcome back banner", "Welcome back, user!\n"},
		// Real Claude Code v2.1.119 layout: the idle prompt sits on its own
		// line ABOVE a status bar (bypass permissions / effort indicator),
		// so the LAST non-empty line is the status bar, not ❯. DetectReady
		// must still fire.
		{"v2+ prompt above status bar", strings.Join([]string{
			"╭─── Claude Code v2.1.119 ────╮",
			"│  Welcome back Hanfour!       │",  // banner would also match, but test a case without it too
			"╰──────────────────────────────╯",
			"",
			"────────────────────────────────",
			"❯ ",
			"────────────────────────────────",
			"  ⏵⏵ bypass permissions on (shift+tab to cycle)   ● high · /effort",
		}, "\n")},
		{"v2+ prompt with status bar no banner",
			"────────────\n❯ \n────────────\n  ⏵⏵ bypass permissions on (shift+tab to cycle)\n"},
	}
	for _, tc := range readyCases {
		t.Run("ready/"+tc.name, func(t *testing.T) {
			if !isClaudeReady(tc.content) {
				t.Errorf("isClaudeReady(%q) = false, want true", tc.content)
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
		{"help without keyword", "i am here to assist"},
		// M3 regression: a "> " prefix sitting DEEP in scrollback (e.g. a
		// quoted response from a previous turn) must NOT fire ready when
		// the actual trailing pane lines are busy output. The fix scans
		// only the last readyScanLines (5) so scrollback beyond that is
		// immune. Each case below pushes the prompt-looking line past the
		// 5-line window so the scan cannot see it.
		{"gt prompt deep in scrollback, busy trail", ">\nfetching context\nrunning grep\nfiltering results\napplying patch\ncompiling..."},
		{"gt+text deep scrollback, busy trail", "> quoted reply from last turn\nfetching context\nrunning grep\nfiltering results\napplying patch\ncompiling..."},
		{"caret deep scrollback, busy trail", "❯\nfetching context\nrunning grep\nfiltering results\napplying patch\nwriting output..."},
		{"caret+text deep scrollback, busy trail", "❯ partial\nfetching context\nrunning grep\nfiltering results\napplying patch\nmore work..."},
	}
	for _, tc := range notReadyCases {
		t.Run("notready/"+tc.name, func(t *testing.T) {
			if isClaudeReady(tc.content) {
				t.Errorf("isClaudeReady(%q) = true, want false", tc.content)
			}
		})
	}
}

// TestClaudeCodeRuntime_ParseCompletionIndicators verifies isClaudeIdle returns
// true ONLY when the last non-empty line is a bare prompt, not when the prompt
// appears earlier in the line with user-entered text.
func TestClaudeCodeRuntime_ParseCompletionIndicators(t *testing.T) {
	idleCases := []struct {
		name    string
		content string
	}{
		{"bare gt", ">"},
		{"gt with trailing space", "> "},
		{"bare caret", "❯"},
		{"caret with trailing space", "❯ "},
		{"multiline ending in caret", "thinking...\ndone.\n❯"},
		{"multiline ending in caret with trailing blank lines", "task complete\n❯\n\n\n"},
		// Real Claude Code v2.1.119 idle layout: status bar sits BELOW the
		// prompt, so the last non-empty line is the status bar text — not ❯.
		// PR #19 fixed this for ready detection; idle detection has the same
		// pane shape and must follow.
		{"v2+ idle prompt above status bar", strings.Join([]string{
			"some output",
			"",
			"────────────────────────────────",
			"❯ ",
			"────────────────────────────────",
			"  ⏵⏵ bypass permissions on (shift+tab to cycle)   ● high · /effort",
		}, "\n")},
		{"v2+ idle no banner only prompt + status",
			"────────────\n❯\n────────────\n  ⏵⏵ bypass permissions on\n"},
	}
	for _, tc := range idleCases {
		t.Run("idle/"+tc.name, func(t *testing.T) {
			if !isClaudeIdle(tc.content) {
				t.Errorf("isClaudeIdle(%q) = false, want true", tc.content)
			}
		})
	}

	busyCases := []struct {
		name    string
		content string
	}{
		{"empty", ""},
		// Strict bare-prompt match means "❯" with user-typed text is NOT idle —
		// the user is composing input, not waiting.
		{"caret with user input on same line", "❯ run the tests"},
		// During real claude generation the prompt area is replaced by
		// progress indicators; bare ❯ is pushed past the trailing window.
		{"prompt deep in scrollback, generating",
			"❯\nfetching files\nrunning grep\nwriting changes\napplying patch\n✻ Crunched for 2m"},
		{"gt prompt deep in scrollback",
			">\nold log line\nmore log\nmore work\nstill more\n✻ Generating..."},
		{"random shell prompt", "user@host:~$"},
		{"progress spinner as last line", "⠋ Thinking..."},
	}
	for _, tc := range busyCases {
		t.Run("busy/"+tc.name, func(t *testing.T) {
			if isClaudeIdle(tc.content) {
				t.Errorf("isClaudeIdle(%q) = true, want false", tc.content)
			}
		})
	}
}

// TestClaudeCodeRuntime_ParseTokenUsage verifies the Runtime.ParseTokenUsage
// helper against representative pane snippets. Convention: when only a single
// aggregate "Total tokens" value is available, it is stored in InputTokens
// and OutputTokens is left at 0.
func TestClaudeCodeRuntime_ParseTokenUsage(t *testing.T) {
	r := New(nil)

	t.Run("total tokens aggregate", func(t *testing.T) {
		usage, err := r.ParseTokenUsage("... Total tokens: 12,345 ...")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := usage.InputTokens + usage.OutputTokens; got != 12345 {
			t.Fatalf("InputTokens+OutputTokens = %d, want 12345 (got usage=%+v)", got, usage)
		}
	})

	t.Run("input and output split", func(t *testing.T) {
		content := "input tokens: 1,000\noutput tokens: 2,345"
		usage, err := r.ParseTokenUsage(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := usage.InputTokens + usage.OutputTokens; got != 3345 {
			t.Fatalf("InputTokens+OutputTokens = %d, want 3345 (got usage=%+v)", got, usage)
		}
	})

	t.Run("total cost only", func(t *testing.T) {
		usage, err := r.ParseTokenUsage("Total cost: $0.1234")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if usage.TotalCost <= 0 {
			t.Fatalf("TotalCost = %v, want > 0 (usage=%+v)", usage.TotalCost, usage)
		}
	})

	t.Run("missing tokens returns zero value", func(t *testing.T) {
		usage, err := r.ParseTokenUsage("no usage metrics here")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if (usage != agent.TokenUsage{}) {
			t.Fatalf("expected zero value TokenUsage, got %+v", usage)
		}
	})
}

// PromptRenderDelay and ShellEscape are now shared helpers in
// internal/agent/runtimeutil/ and tested there.
