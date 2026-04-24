// Package runtimeutil collects small helpers shared across AgentRuntime
// plugin implementations (claudecode, aisagent, aider, ...). Keeping them in
// one place prevents drift — if the tmux send-keys quoting rules or
// Claude-style token parsing change, they only need to be updated once.
package runtimeutil

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ShellEscape wraps s in single quotes, safely escaping any embedded single
// quotes. The returned string is a single shell word suitable for appending
// to a command line sent via tmux send-keys.
//
// The empty string produces `''` — a valid empty argument.
func ShellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// PromptRenderDelay returns how long to wait after SendLiteralKeys before
// pressing Enter. Scales with prompt length: 1s base + 500ms per 2000 chars,
// capped at 5s. Short prompts finish rendering quickly; very long ones need
// extra time for the CLI to echo the pasted text before Enter is accepted.
func PromptRenderDelay(promptLen int) time.Duration {
	base := 1 * time.Second
	extra := time.Duration(promptLen/2000) * 500 * time.Millisecond
	total := base + extra
	if total > 5*time.Second {
		total = 5 * time.Second
	}
	return total
}

// NewSessionName builds a unique tmux session identifier of the form
// "<prefix>-<unix-nanos>-<hex4>". The random suffix guards against
// collisions when two spawns happen in the same nanosecond.
func NewSessionName(prefix string) (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixNano(), hex.EncodeToString(buf[:])), nil
}

// ParseTokenNum parses a Claude-style token count string like "12,345" or
// "1_000_000" by stripping separators and converting to int. Invalid input
// returns 0 without error — callers treat missing data as zero.
func ParseTokenNum(s string) int {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "_", "")
	val, _ := strconv.Atoi(s)
	return val
}

// Shared regexes for Claude Code / ais-agent pane-output token reporting.
// Kept here so all plugins that accept Claude-style telemetry stay in sync.
var (
	// TotalTokensRe matches "Total tokens: 12,345" (case-insensitive).
	TotalTokensRe = regexp.MustCompile(`(?i)total\s+tokens?(?:\s+used)?[:\s]+([0-9][0-9,_]+)`)
	// IOTokensRe matches "input tokens: 12,345" or "output tokens: 6,789".
	IOTokensRe = regexp.MustCompile(`(?i)(input|output)\s+tokens?[:\s]+([0-9][0-9,_]+)`)
	// CostRe matches "Total cost: $0.1234".
	CostRe = regexp.MustCompile(`(?i)total\s+cost[:\s]+\$([0-9]+\.?[0-9]*)`)
)
