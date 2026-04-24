package runtimeutil

import (
	"strings"
	"testing"
	"time"
)

func TestShellEscape(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "''"},
		{"simple", "'simple'"},
		{"has space", "'has space'"},
		{"don't", "'don'\\''t'"},
		{"/tmp/a b/c", "'/tmp/a b/c'"},
	}
	for _, c := range cases {
		if got := ShellEscape(c.in); got != c.want {
			t.Errorf("ShellEscape(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPromptRenderDelay(t *testing.T) {
	cases := []struct {
		promptLen int
		want      time.Duration
	}{
		{0, 1 * time.Second},
		{1999, 1 * time.Second},
		{2000, 1500 * time.Millisecond},
		{4000, 2 * time.Second},
		{16000, 5 * time.Second},   // cap
		{1_000_000, 5 * time.Second}, // cap holds
	}
	for _, c := range cases {
		if got := PromptRenderDelay(c.promptLen); got != c.want {
			t.Errorf("PromptRenderDelay(%d) = %v, want %v", c.promptLen, got, c.want)
		}
	}
}

func TestNewSessionName_PrefixAndUniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 50)
	for i := 0; i < 50; i++ {
		s, err := NewSessionName("test")
		if err != nil {
			t.Fatalf("NewSessionName err: %v", err)
		}
		if !strings.HasPrefix(s, "test-") {
			t.Errorf("name missing prefix: %q", s)
		}
		if _, dup := seen[s]; dup {
			t.Errorf("duplicate session name generated: %q", s)
		}
		seen[s] = struct{}{}
	}
}

func TestParseTokenNum(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"0", 0},
		{"12,345", 12345},
		{"1_000_000", 1000000},
		{"abc", 0},
	}
	for _, c := range cases {
		if got := ParseTokenNum(c.in); got != c.want {
			t.Errorf("ParseTokenNum(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestTokenRegexes(t *testing.T) {
	if m := TotalTokensRe.FindStringSubmatch("Total tokens: 12,345"); len(m) != 2 || m[1] != "12,345" {
		t.Errorf("TotalTokensRe bad match: %v", m)
	}
	if m := IOTokensRe.FindAllStringSubmatch("input tokens: 100\noutput tokens: 200", -1); len(m) != 2 {
		t.Errorf("IOTokensRe expected 2 matches, got %d: %v", len(m), m)
	}
	if m := CostRe.FindStringSubmatch("Total cost: $0.1234"); len(m) != 2 || m[1] != "0.1234" {
		t.Errorf("CostRe bad match: %v", m)
	}
}
