package codex

import (
	"strings"
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/agent"
)

func TestRuntime_Name(t *testing.T) {
	r := New(nil)
	if got := r.Name(); got != "codex" {
		t.Errorf("Name() = %q, want %q", got, "codex")
	}
}

func TestRuntime_MonitoredSessionType(t *testing.T) {
	r := New(nil)
	if got := r.MonitoredSessionType(); got != "codex" {
		t.Errorf("MonitoredSessionType() = %q, want %q", got, "codex")
	}
}

// Validate is a no-op — Codex's CLI flags are constructed by us, not
// passed through from cfg, so there's no impossible-config to fail
// fast on.
func TestRuntime_Validate_AlwaysNil(t *testing.T) {
	r := New(nil)
	if err := r.Validate(agent.SpawnConfig{}); err != nil {
		t.Errorf("Validate(empty) returned %v, want nil", err)
	}
	if err := r.Validate(agent.SpawnConfig{Model: "gpt-99", PermissionMode: "garbage"}); err != nil {
		t.Errorf("Validate(garbage) returned %v, want nil (validation happens at DetectReady)", err)
	}
}

// =============================================================
// buildCLICommand
// =============================================================

func TestBuildCLICommand_AlwaysEmitsNoAltScreen(t *testing.T) {
	// Without --no-alt-screen, tmux capture-pane sees an empty pane.
	// Every other detection signal hinges on this flag, so it must
	// fire on EVERY invocation including empty cfg.
	cmd := buildCLICommand(agent.SpawnConfig{})
	if !strings.Contains(cmd, "--no-alt-screen") {
		t.Errorf("buildCLICommand(empty) missing --no-alt-screen: %s", cmd)
	}
}

func TestBuildCLICommand_PermissionMode(t *testing.T) {
	cases := []struct {
		mode         string
		wantContains []string
		notContains  []string
	}{
		{
			mode:         "bypassPermissions",
			wantContains: []string{"--dangerously-bypass-approvals-and-sandbox"},
			notContains:  []string{"--sandbox", "--ask-for-approval"},
		},
		{
			mode:         "acceptEdits",
			wantContains: []string{"--sandbox workspace-write", "--ask-for-approval on-request"},
			notContains:  []string{"--dangerously-bypass", "read-only"},
		},
		{
			mode:         "plan",
			wantContains: []string{"--sandbox read-only", "--ask-for-approval on-request"},
			notContains:  []string{"--dangerously-bypass", "workspace-write"},
		},
		{
			mode:         "", // unset → safest default
			wantContains: []string{"--ask-for-approval untrusted"},
			notContains:  []string{"--dangerously-bypass", "--sandbox"},
		},
		{
			mode:         "unknown-policy",
			wantContains: []string{"--ask-for-approval untrusted"}, // fall through
			notContains:  []string{"--dangerously-bypass"},
		},
	}
	for _, tc := range cases {
		t.Run("mode="+tc.mode, func(t *testing.T) {
			cmd := buildCLICommand(agent.SpawnConfig{PermissionMode: tc.mode})
			for _, want := range tc.wantContains {
				if !strings.Contains(cmd, want) {
					t.Errorf("missing %q in: %s", want, cmd)
				}
			}
			for _, bad := range tc.notContains {
				if strings.Contains(cmd, bad) {
					t.Errorf("unexpected %q in: %s", bad, cmd)
				}
			}
		})
	}
}

func TestBuildCLICommand_ModelAndWorkdir(t *testing.T) {
	cmd := buildCLICommand(agent.SpawnConfig{
		Model:   "gpt-5.5",
		WorkDir: "/tmp/work space",
	})
	if !strings.Contains(cmd, "-m gpt-5.5") {
		t.Errorf("missing model flag: %s", cmd)
	}
	if !strings.Contains(cmd, "-C '/tmp/work space'") {
		t.Errorf("missing or unescaped workdir flag: %s", cmd)
	}
}

func TestBuildCLICommand_ExtraCLIArgsAppendedVerbatim(t *testing.T) {
	// Trusted config — not shell-escaped — appended as-is so callers
	// can pass pre-formed flag strings.
	cmd := buildCLICommand(agent.SpawnConfig{
		ExtraCLIArgs: "--enable some-feature --search",
	})
	if !strings.HasSuffix(cmd, "--enable some-feature --search") {
		t.Errorf("ExtraCLIArgs not at tail of: %s", cmd)
	}
}

// =============================================================
// isCodexReady / isCodexIdle
// =============================================================

func TestIsCodexReady_ReadyCases(t *testing.T) {
	readyCases := []struct {
		name    string
		content string
	}{
		{"banner header alone", "OpenAI Codex (v0.130.0)"},
		{"banner inside box drawing", "╭───╮\n│ >_ OpenAI Codex (v0.130.0) │\n╰───╯"},
		{"bare arrow prompt", "›"},
		{"arrow prompt with space", "› "},
		{"arrow prompt with user text", "› hello\n"},
		// Real Codex layout: arrow prompt sits one line above the
		// model/path footer, so the LAST non-empty line is the footer.
		{"arrow above footer", strings.Join([]string{
			"some history above",
			"",
			"› ",
			"",
			"  gpt-5.5 xhigh · /Volumes/SATECHI DISK Media/UserFolders/Projects/bamboo",
		}, "\n")},
	}
	for _, tc := range readyCases {
		t.Run("ready/"+tc.name, func(t *testing.T) {
			if !isCodexReady(tc.content) {
				t.Errorf("isCodexReady(%q) = false, want true", tc.content)
			}
		})
	}
}

func TestIsCodexReady_NotReadyCases(t *testing.T) {
	notReady := []struct {
		name    string
		content string
	}{
		{"empty", ""},
		{"shell prompt only", "user@host:~$"},
		{"loading message", "Loading workspace..."},
		// Claude Code prompts must NOT be misread as Codex prompts.
		{"claude caret prompt", "❯"},
		{"gt prompt", "> "},
		// Arrow prompt deep in scrollback should NOT fire if the
		// trailing 5-line window doesn't include it.
		{"arrow deep in scrollback", strings.Join([]string{
			"›",
			"line2", "line3", "line4", "line5", "line6", "line7",
		}, "\n")},
	}
	for _, tc := range notReady {
		t.Run("notready/"+tc.name, func(t *testing.T) {
			if isCodexReady(tc.content) {
				t.Errorf("isCodexReady(%q) = true, want false", tc.content)
			}
		})
	}
}

func TestIsCodexIdle_IdleCases(t *testing.T) {
	idleCases := []struct {
		name    string
		content string
	}{
		{"bare arrow", "›"},
		{"arrow with space", "› "},
		{"arrow above footer (real layout)", strings.Join([]string{
			"some history above",
			"",
			"› ",
			"",
			"  gpt-5.5 xhigh · /Volumes/SATECHI DISK Media/UserFolders/Projects/bamboo",
		}, "\n")},
	}
	for _, tc := range idleCases {
		t.Run("idle/"+tc.name, func(t *testing.T) {
			if !isCodexIdle(tc.content) {
				t.Errorf("isCodexIdle(%q) = false, want true", tc.content)
			}
		})
	}
}

func TestIsCodexIdle_NotIdleCases(t *testing.T) {
	notIdle := []struct {
		name    string
		content string
	}{
		{"empty", ""},
		{"thinking spinner", "⠋ thinking..."},
		// Arrow with user-typed continuation is NOT idle — they're
		// in mid-prompt entry.
		{"arrow with user text", "› hello world"},
		// Arrow deep in scrollback past the 5-line window should not
		// fire idle when the tail is busy output.
		{"arrow deep, busy trail", strings.Join([]string{
			"›",
			"fetching context",
			"running grep",
			"writing patch",
			"compiling tests",
			"running benchmarks",
		}, "\n")},
	}
	for _, tc := range notIdle {
		t.Run("notidle/"+tc.name, func(t *testing.T) {
			if isCodexIdle(tc.content) {
				t.Errorf("isCodexIdle(%q) = true, want false", tc.content)
			}
		})
	}
}

// ParseTokenUsage is documented as a no-op for this runtime.
func TestRuntime_ParseTokenUsage_ZeroValue(t *testing.T) {
	r := New(nil)
	usage, err := r.ParseTokenUsage("Tokens: 1000 sent, 500 received, $0.05 cost")
	if err != nil {
		t.Fatalf("ParseTokenUsage err = %v", err)
	}
	if (usage != agent.TokenUsage{}) {
		t.Errorf("expected zero TokenUsage, got %+v", usage)
	}
}
