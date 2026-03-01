package detector

import (
	"regexp"
	"strings"
)

var (
	claudeProceedRe  = regexp.MustCompile(`(?i)Do you want to proceed\?`)
	claudeOptionRe   = regexp.MustCompile(`[❯>\s]+(\d+)\.\s+(.+)`)
	claudeBoxTopRe   = regexp.MustCompile(`[╭┌][-─]+[╮┐]`)
	claudeBoxBotRe   = regexp.MustCompile(`[╰└][-─]+[╯┘]`)
	claudeAllowRe    = regexp.MustCompile(`(?i)Allow\s+(once|always)`)
	claudeDenyRe     = regexp.MustCompile(`(?i)Deny`)
)

type ClaudeDetector struct{}

func NewClaudeDetector() *ClaudeDetector {
	return &ClaudeDetector{}
}

func (d *ClaudeDetector) Name() string     { return "claude_code" }
func (d *ClaudeDetector) Priority() int    { return 10 }

func (d *ClaudeDetector) Detect(paneContent string) (*PromptMatch, bool) {
	lines := strings.Split(paneContent, "\n")

	// Search from the bottom up for the prompt
	lastLines := lines
	if len(lastLines) > 40 {
		lastLines = lastLines[len(lastLines)-40:]
	}
	content := strings.Join(lastLines, "\n")

	// Check for "Do you want to proceed?" pattern
	if claudeProceedRe.MatchString(content) {
		return d.parseProceedPrompt(content, lastLines), true
	}

	// Check for Allow/Deny pattern (tool use permission)
	if claudeAllowRe.MatchString(content) && claudeDenyRe.MatchString(content) {
		return d.parseAllowDenyPrompt(content, lastLines), true
	}

	return nil, false
}

func (d *ClaudeDetector) parseProceedPrompt(content string, lines []string) *PromptMatch {
	match := &PromptMatch{
		Type:        PromptTypeClaudeCode,
		FullContext:  content,
	}

	// Extract options
	for _, line := range lines {
		if m := claudeOptionRe.FindStringSubmatch(line); len(m) >= 3 {
			match.Options = append(match.Options, ResponseOption{
				Key:   m[1],
				Label: strings.TrimSpace(m[2]),
			})
		}
	}

	// If no numbered options found, provide defaults
	if len(match.Options) == 0 {
		match.Options = []ResponseOption{
			{Key: "1", Label: "Yes"},
			{Key: "2", Label: "Yes, and don't ask again"},
			{Key: "3", Label: "No, and tell Claude"},
		}
	}

	// Extract summary: look for the action description above the prompt
	match.Summary = d.extractSummary(lines)

	return match
}

func (d *ClaudeDetector) parseAllowDenyPrompt(content string, lines []string) *PromptMatch {
	match := &PromptMatch{
		Type:        PromptTypeClaudeCode,
		FullContext:  content,
		Options: []ResponseOption{
			{Key: "y", Label: "Allow"},
			{Key: "n", Label: "Deny"},
		},
	}

	// Try to find numbered options
	for _, line := range lines {
		if m := claudeOptionRe.FindStringSubmatch(line); len(m) >= 3 {
			match.Options = append(match.Options[:0], ResponseOption{
				Key:   m[1],
				Label: strings.TrimSpace(m[2]),
			})
		}
	}

	match.Summary = d.extractSummary(lines)
	return match
}

func (d *ClaudeDetector) extractSummary(lines []string) string {
	// Look for content between box borders
	inBox := false
	var summaryLines []string
	for _, line := range lines {
		if claudeBoxTopRe.MatchString(line) {
			inBox = true
			continue
		}
		if claudeBoxBotRe.MatchString(line) {
			inBox = false
			continue
		}
		if inBox {
			cleaned := strings.TrimLeft(line, "│┃| ")
			cleaned = strings.TrimRight(cleaned, "│┃| ")
			cleaned = strings.TrimSpace(cleaned)
			if cleaned != "" {
				summaryLines = append(summaryLines, cleaned)
			}
		}
	}
	if len(summaryLines) > 0 {
		return strings.Join(summaryLines, " | ")
	}

	// Fallback: use the line before "Do you want to proceed?"
	for i, line := range lines {
		if claudeProceedRe.MatchString(line) && i > 0 {
			return strings.TrimSpace(lines[i-1])
		}
	}
	return "Claude Code permission prompt"
}
