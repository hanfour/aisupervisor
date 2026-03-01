package observer

import (
	"regexp"
	"strings"
	"time"
)

var errorPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\berror\b.*:`),
	regexp.MustCompile(`(?i)\bFAIL\b`),
	regexp.MustCompile(`(?i)\bpanic\b:`),
	regexp.MustCompile(`(?i)traceback \(most recent`),
	regexp.MustCompile(`(?i)exception in thread`),
}

var inputReadyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`❯\s*$`),
	regexp.MustCompile(`\$\s*$`),
	regexp.MustCompile(`>\s*$`),
}

// ContentObserver detects errors, idle states, and input-ready prompts.
type ContentObserver struct {
	idleThreshold   time.Duration
	lastChangeTime  time.Time
	lastContent     string
	idleNotified    bool
}

// NewContentObserver creates a content observer with the given idle threshold.
func NewContentObserver(idleThreshold time.Duration) *ContentObserver {
	return &ContentObserver{
		idleThreshold:  idleThreshold,
		lastChangeTime: time.Now(),
	}
}

func (o *ContentObserver) Observe(content, prevContent string) []ObservationEvent {
	var events []ObservationEvent

	contentChanged := content != prevContent
	if contentChanged {
		o.lastChangeTime = time.Now()
		o.lastContent = content
		o.idleNotified = false
	}

	// Error detection: only on new content
	if contentChanged {
		newLines := extractNewLines(content, prevContent)
		for _, line := range newLines {
			for _, re := range errorPatterns {
				if re.MatchString(line) {
					events = append(events, ObservationEvent{
						Type:        ObservationError,
						PaneContent: content,
					})
					goto doneErrors // one error event per observation
				}
			}
		}
	}
doneErrors:

	// Idle detection
	if !o.idleNotified && o.idleThreshold > 0 && time.Since(o.lastChangeTime) > o.idleThreshold {
		events = append(events, ObservationEvent{
			Type:        ObservationIdle,
			PaneContent: content,
		})
		o.idleNotified = true
	}

	// Input ready detection (only on change)
	if contentChanged {
		lastLine := lastNonEmptyLine(content)
		for _, re := range inputReadyPatterns {
			if re.MatchString(lastLine) {
				events = append(events, ObservationEvent{
					Type:        ObservationInputReady,
					PaneContent: content,
				})
				break
			}
		}
	}

	return events
}

func extractNewLines(content, prevContent string) []string {
	if prevContent == "" {
		return strings.Split(content, "\n")
	}
	// Simple: lines in content that aren't in prevContent
	prevLines := strings.Split(prevContent, "\n")
	curLines := strings.Split(content, "\n")

	prevSet := make(map[string]bool, len(prevLines))
	for _, l := range prevLines {
		prevSet[l] = true
	}

	var newLines []string
	for _, l := range curLines {
		if !prevSet[l] {
			newLines = append(newLines, l)
		}
	}
	return newLines
}

func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			return lines[i]
		}
	}
	return ""
}
