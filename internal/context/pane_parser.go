package context

import (
	"regexp"
	"strings"
)

var (
	// Match common shell prompts that show the cwd, e.g.:
	//   user@host:/some/path$
	//   /some/path $
	//   ~/projects/foo ❯
	promptCwdRe = regexp.MustCompile(`(?m)(?:^|[\s:])((?:/[\w.@-]+)+(?:/[\w.@-]*)*)[$❯%#>\s]`)

	// Match explicit cd commands: cd /path or cd ~/path
	cdCmdRe = regexp.MustCompile(`(?m)\$\s*cd\s+(~?/[\w./@-]+)`)

	// Match pwd output: a line that is just an absolute path
	pwdOutputRe = regexp.MustCompile(`(?m)^(/[\w./@-]+)$`)

	// Match shell command lines (lines starting with $ or ❯)
	cmdLineRe = regexp.MustCompile(`(?m)^[\s]*[$❯%#>]\s*(.+)$`)
)

// ParseWorkingDirectory attempts to extract the current working directory
// from pane content by looking for pwd output, cd commands, or shell prompts.
func ParseWorkingDirectory(content string) string {
	lines := strings.Split(content, "\n")

	// Scan from bottom to top for the most recent signal
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Check cd command
		if m := cdCmdRe.FindStringSubmatch(line); len(m) > 1 {
			return expandHome(m[1])
		}

		// Check pwd output (standalone absolute path)
		if m := pwdOutputRe.FindStringSubmatch(line); len(m) > 1 {
			// Verify it looks like a directory, not a file path in output
			if !strings.Contains(line, " ") {
				return m[1]
			}
		}

		// Check prompt with cwd
		if m := promptCwdRe.FindStringSubmatch(line); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}

// SummarizeActivity extracts a brief summary of recent pane activity.
// It collects command lines and truncates to maxLen characters.
func SummarizeActivity(content string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 500
	}

	matches := cmdLineRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		// Fallback: take the last few non-empty lines
		return lastLines(content, 5, maxLen)
	}

	var commands []string
	for _, m := range matches {
		cmd := strings.TrimSpace(m[1])
		if cmd != "" {
			commands = append(commands, cmd)
		}
	}

	// Keep the most recent commands
	result := strings.Join(commands, "\n")
	if len(result) > maxLen {
		result = result[len(result)-maxLen:]
		// Trim to the nearest newline for cleanliness
		if idx := strings.Index(result, "\n"); idx >= 0 {
			result = result[idx+1:]
		}
	}
	return result
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		// We can't reliably expand ~ here, return as-is
		return path
	}
	return path
}

func lastLines(content string, n, maxLen int) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	start := len(lines) - n
	if start < 0 {
		start = 0
	}
	result := strings.Join(lines[start:], "\n")
	if len(result) > maxLen {
		result = result[len(result)-maxLen:]
	}
	return result
}
