package observer

import "github.com/hanfourmini/aisupervisor/internal/detector"

// ObservationType classifies observation events.
type ObservationType string

const (
	ObservationPrompt         ObservationType = "prompt"
	ObservationContentChanged ObservationType = "content_changed"
	ObservationError          ObservationType = "error"
	ObservationIdle           ObservationType = "idle"
	ObservationInputReady     ObservationType = "input_ready"
)

// ObservationEvent represents something observed in a pane.
type ObservationEvent struct {
	Type        ObservationType
	PaneContent string
	Prompt      *detector.PromptMatch
	SessionID   string
}

// Observer detects events from pane content changes.
type Observer interface {
	Observe(content, prevContent string) []ObservationEvent
}
