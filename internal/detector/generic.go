package detector

import (
	"regexp"
	"strings"
)

var (
	genericYNRe       = regexp.MustCompile(`(?i)\[y/n\]|\(y/n\)|yes/no|confirm\?`)
	genericContinueRe = regexp.MustCompile(`(?i)continue\?\s*\[y/n\]|proceed\?\s*\(y/n\)`)
)

type GenericDetector struct{}

func NewGenericDetector() *GenericDetector {
	return &GenericDetector{}
}

func (d *GenericDetector) Name() string     { return "generic" }
func (d *GenericDetector) Priority() int    { return 100 }

func (d *GenericDetector) Detect(paneContent string) (*PromptMatch, bool) {
	lines := strings.Split(paneContent, "\n")

	lastLines := lines
	if len(lastLines) > 10 {
		lastLines = lastLines[len(lastLines)-10:]
	}
	content := strings.Join(lastLines, "\n")

	if !genericYNRe.MatchString(content) {
		return nil, false
	}

	// Find the specific prompt line
	var promptLine string
	for i := len(lastLines) - 1; i >= 0; i-- {
		if genericYNRe.MatchString(lastLines[i]) {
			promptLine = strings.TrimSpace(lastLines[i])
			break
		}
	}

	return &PromptMatch{
		Type:        PromptTypeGeneric,
		Summary:     promptLine,
		FullContext:  content,
		Options: []ResponseOption{
			{Key: "y", Label: "Yes"},
			{Key: "n", Label: "No"},
		},
	}, true
}
