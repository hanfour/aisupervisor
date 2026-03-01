package detector

import (
	"regexp"
	"strings"
)

var (
	geminiYesNoRe    = regexp.MustCompile(`\(Y\)es\s*/\s*\(N\)o`)
	geminiShellCmdRe = regexp.MustCompile(`(?i)Run shell command\?`)
	geminiModifyRe   = regexp.MustCompile(`\(M\)odify`)
)

type GeminiDetector struct{}

func NewGeminiDetector() *GeminiDetector {
	return &GeminiDetector{}
}

func (d *GeminiDetector) Name() string     { return "gemini" }
func (d *GeminiDetector) Priority() int    { return 20 }

func (d *GeminiDetector) Detect(paneContent string) (*PromptMatch, bool) {
	lines := strings.Split(paneContent, "\n")

	lastLines := lines
	if len(lastLines) > 30 {
		lastLines = lastLines[len(lastLines)-30:]
	}
	content := strings.Join(lastLines, "\n")

	if !geminiYesNoRe.MatchString(content) {
		return nil, false
	}

	match := &PromptMatch{
		Type:        PromptTypeGemini,
		FullContext:  content,
		Options: []ResponseOption{
			{Key: "Y", Label: "Yes"},
			{Key: "N", Label: "No"},
		},
	}

	if geminiModifyRe.MatchString(content) {
		match.Options = append(match.Options, ResponseOption{
			Key:   "M",
			Label: "Modify",
		})
	}

	// Extract command being asked about
	match.Summary = d.extractCommand(lastLines)

	return match, true
}

func (d *GeminiDetector) extractCommand(lines []string) string {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "$") {
			return "Run: " + strings.TrimSpace(strings.TrimPrefix(trimmed, "$"))
		}
	}

	if geminiShellCmdRe.MatchString(strings.Join(lines, "\n")) {
		return "Gemini shell command prompt"
	}
	return "Gemini confirmation prompt"
}
