package observer

import "github.com/hanfourmini/aisupervisor/internal/detector"

// PromptObserver wraps the existing detector registry to produce prompt observation events.
type PromptObserver struct {
	registry *detector.Registry
}

// NewPromptObserver creates a prompt observer using the given detector registry.
func NewPromptObserver(registry *detector.Registry) *PromptObserver {
	return &PromptObserver{registry: registry}
}

func (o *PromptObserver) Observe(content, prevContent string) []ObservationEvent {
	match, ok := o.registry.Detect(content)
	if !ok {
		return nil
	}

	return []ObservationEvent{
		{
			Type:        ObservationPrompt,
			PaneContent: content,
			Prompt:      match,
		},
	}
}
