package aisagent

import (
	"strings"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/agent"
)

// TestAISAgentRuntime_Name verifies the runtime self-identifies as "ais-agent".
func TestAISAgentRuntime_Name(t *testing.T) {
	r := New(nil)
	if got := r.Name(); got != "ais-agent" {
		t.Fatalf("Name() = %q, want %q", got, "ais-agent")
	}
}

// TestAISAgentRuntime_InterfaceSatisfied ensures *Runtime implements agent.AgentRuntime.
func TestAISAgentRuntime_InterfaceSatisfied(t *testing.T) {
	var _ agent.AgentRuntime = (*Runtime)(nil)
}

// TestAISAgentRuntime_BuildCLICommand verifies that a populated SpawnConfig is
// translated into the expected CLI flags for the ais-agent binary.
//
// ais-agent flag conventions (differ from Claude Code):
//   - --allowed-tools / --disallowed-tools take a single COMMA-SEPARATED value
//     (one flag + one argument), NOT multiple shell-escaped tokens.
//   - Permission is controlled by --permission-mode only; there is no
//     --dangerously-skip-permissions counterpart.
//   - Provider comes from EnvVars["AIS_PROVIDER"].
//   - Max tokens comes from EnvVars["AIS_MAX_TOKENS"] (string → int).
func TestAISAgentRuntime_BuildCLICommand(t *testing.T) {
	cfg := agent.SpawnConfig{
		Model:           "claude-sonnet-4-5",
		PermissionMode:  "acceptEdits",
		SystemPrompt:    "You are a helpful assistant.",
		AllowedTools:    []string{"Bash", "Edit", "Read"},
		DisallowedTools: []string{"Skill", "EnterPlanMode"},
		ExtraCLIArgs:    "--verbose",
		EnvVars: map[string]string{
			"AIS_PROVIDER":   "anthropic",
			"AIS_MAX_TOKENS": "8192",
		},
	}

	cmd := buildCLICommand(cfg)

	// Must start with the ais-agent binary.
	if !strings.HasPrefix(cmd, "ais-agent ") {
		t.Fatalf("buildCLICommand: expected binary prefix %q, got %q", "ais-agent", cmd)
	}

	// --provider <p> (from EnvVars["AIS_PROVIDER"])
	if !strings.Contains(cmd, "--provider anthropic") {
		t.Errorf("buildCLICommand: missing --provider flag, got %q", cmd)
	}

	// --model <m>
	if !strings.Contains(cmd, "--model claude-sonnet-4-5") {
		t.Errorf("buildCLICommand: missing --model flag, got %q", cmd)
	}

	// --permission-mode <mode> — ais-agent does NOT use --dangerously-skip-permissions.
	if !strings.Contains(cmd, "--permission-mode acceptEdits") {
		t.Errorf("buildCLICommand: missing --permission-mode flag, got %q", cmd)
	}
	if strings.Contains(cmd, "--dangerously-skip-permissions") {
		t.Errorf("buildCLICommand: unexpected --dangerously-skip-permissions, got %q", cmd)
	}

	// --allowed-tools <csv> — single flag, one comma-separated argument.
	if !strings.Contains(cmd, "--allowed-tools Bash,Edit,Read") {
		t.Errorf("buildCLICommand: expected comma-separated --allowed-tools, got %q", cmd)
	}

	// --disallowed-tools <csv>
	if !strings.Contains(cmd, "--disallowed-tools Skill,EnterPlanMode") {
		t.Errorf("buildCLICommand: expected comma-separated --disallowed-tools, got %q", cmd)
	}

	// --append-system-prompt <shell-escaped>
	if !strings.Contains(cmd, "--append-system-prompt 'You are a helpful assistant.'") {
		t.Errorf("buildCLICommand: expected shell-escaped system prompt, got %q", cmd)
	}

	// --max-tokens <n> (from EnvVars["AIS_MAX_TOKENS"])
	if !strings.Contains(cmd, "--max-tokens 8192") {
		t.Errorf("buildCLICommand: expected --max-tokens 8192, got %q", cmd)
	}

	// ExtraCLIArgs appended verbatim at the end.
	if !strings.HasSuffix(cmd, "--verbose") {
		t.Errorf("buildCLICommand: expected ExtraCLIArgs trailing the command, got %q", cmd)
	}
}

// TestAISAgentRuntime_BuildCLICommand_EmptyConfig verifies the no-op path.
func TestAISAgentRuntime_BuildCLICommand_EmptyConfig(t *testing.T) {
	cmd := buildCLICommand(agent.SpawnConfig{})
	if strings.TrimSpace(cmd) != "ais-agent" {
		t.Errorf("empty cfg: expected %q, got %q", "ais-agent", cmd)
	}
}

// TestAISAgentRuntime_BuildCLICommand_ProviderMissing verifies that without
// AIS_PROVIDER the --provider flag is omitted.
func TestAISAgentRuntime_BuildCLICommand_ProviderMissing(t *testing.T) {
	cfg := agent.SpawnConfig{
		Model: "gpt-4o",
	}
	cmd := buildCLICommand(cfg)
	if strings.Contains(cmd, "--provider") {
		t.Errorf("buildCLICommand: expected no --provider flag when EnvVars[AIS_PROVIDER] unset, got %q", cmd)
	}
	if !strings.Contains(cmd, "--model gpt-4o") {
		t.Errorf("buildCLICommand: expected --model gpt-4o, got %q", cmd)
	}
}

// TestAISAgentRuntime_BuildCLICommand_MaxTokensInvalid verifies that invalid
// AIS_MAX_TOKENS values do NOT emit the flag.
func TestAISAgentRuntime_BuildCLICommand_MaxTokensInvalid(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"non-numeric", "banana"},
		{"zero", "0"},
		{"negative", "-100"},
		{"empty", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := agent.SpawnConfig{
				EnvVars: map[string]string{"AIS_MAX_TOKENS": tc.value},
			}
			cmd := buildCLICommand(cfg)
			if strings.Contains(cmd, "--max-tokens") {
				t.Errorf("expected no --max-tokens for value %q, got %q", tc.value, cmd)
			}
		})
	}
}

// TestAISAgentRuntime_BuildCLICommand_SystemPromptEscaping verifies single-quote
// escaping in the system prompt.
func TestAISAgentRuntime_BuildCLICommand_SystemPromptEscaping(t *testing.T) {
	cfg := agent.SpawnConfig{
		SystemPrompt: "don't panic",
	}
	cmd := buildCLICommand(cfg)
	want := `--append-system-prompt 'don'\''t panic'`
	if !strings.Contains(cmd, want) {
		t.Errorf("escaping: expected %q in %q", want, cmd)
	}
}

// TestAISAgentRuntime_ParseReadyIndicators verifies isAISAgentReady matches all
// documented ready patterns and rejects unrelated content.
//
// ais-agent ready signals:
//   - bare ">" or line starting with "> "
//   - bare "ais>" or line starting with "ais> "
//   - content containing the "Ready" banner (e.g. "ais-agent Ready",
//     "Ready for input", "Agent ready.")
func TestAISAgentRuntime_ParseReadyIndicators(t *testing.T) {
	readyCases := []struct {
		name    string
		content string
	}{
		{"bare gt prompt", ">"},
		{"gt prompt with space", "> "},
		{"bare ais prompt", "ais>"},
		{"ais prompt with space", "ais> "},
		{"gt prompt on its own line", "initializing…\n>\n"},
		{"gt prompt with trailing text", "> what now\n"},
		{"ais prompt on its own line", "starting up\nais>\n"},
		{"ais prompt with trailing text", "ais> type here\n"},
		{"ready banner", "ais-agent v0.1.0\nReady for input.\n"},
		{"agent ready banner", "Agent ready.\n"},
	}
	for _, tc := range readyCases {
		t.Run("ready/"+tc.name, func(t *testing.T) {
			if !isAISAgentReady(tc.content) {
				t.Errorf("isAISAgentReady(%q) = false, want true", tc.content)
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
			if isAISAgentReady(tc.content) {
				t.Errorf("isAISAgentReady(%q) = true, want false", tc.content)
			}
		})
	}
}

// TestAISAgentRuntime_ParseCompletionIndicators verifies isAISAgentIdle returns
// true ONLY when the last non-empty line is a bare prompt (">"/ "> " / "ais>"/
// "ais> "), not when the prompt appears earlier in the buffer followed by text.
func TestAISAgentRuntime_ParseCompletionIndicators(t *testing.T) {
	idleCases := []struct {
		name    string
		content string
	}{
		{"bare gt", ">"},
		{"gt with trailing space", "> "},
		{"bare ais", "ais>"},
		{"ais with trailing space", "ais> "},
		{"multiline ending in ais", "thinking...\ndone.\nais>"},
		{"multiline ending in gt", "task complete\n>\n\n\n"},
	}
	for _, tc := range idleCases {
		t.Run("idle/"+tc.name, func(t *testing.T) {
			if !isAISAgentIdle(tc.content) {
				t.Errorf("isAISAgentIdle(%q) = false, want true", tc.content)
			}
		})
	}

	busyCases := []struct {
		name    string
		content string
	}{
		{"empty", ""},
		{"ais with user input on same line", "ais> run the tests"},
		{"ais earlier in content, text after", "ais>\nstill thinking"},
		{"gt earlier in content, text after", ">\nresponse in progress"},
		{"random shell prompt", "user@host:~$"},
		{"progress spinner as last line", "⠋ Thinking..."},
	}
	for _, tc := range busyCases {
		t.Run("busy/"+tc.name, func(t *testing.T) {
			if isAISAgentIdle(tc.content) {
				t.Errorf("isAISAgentIdle(%q) = true, want false", tc.content)
			}
		})
	}
}

// TestAISAgentRuntime_ParseTokenUsage mirrors the claudecode tests because
// ais-agent emits identical telemetry patterns.
func TestAISAgentRuntime_ParseTokenUsage(t *testing.T) {
	r := New(nil)

	t.Run("total tokens aggregate", func(t *testing.T) {
		usage, err := r.ParseTokenUsage("... Total tokens: 12,345 ...")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := usage.InputTokens + usage.OutputTokens; got != 12345 {
			t.Fatalf("InputTokens+OutputTokens = %d, want 12345 (usage=%+v)", got, usage)
		}
	})

	t.Run("input and output split", func(t *testing.T) {
		content := "input tokens: 1,000\noutput tokens: 2,345"
		usage, err := r.ParseTokenUsage(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := usage.InputTokens + usage.OutputTokens; got != 3345 {
			t.Fatalf("InputTokens+OutputTokens = %d, want 3345 (usage=%+v)", got, usage)
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
