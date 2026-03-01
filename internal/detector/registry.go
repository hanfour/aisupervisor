package detector

import "sort"

type Registry struct {
	detectors []Detector
}

func NewRegistry(detectors ...Detector) *Registry {
	r := &Registry{detectors: detectors}
	sort.Slice(r.detectors, func(i, j int) bool {
		return r.detectors[i].Priority() < r.detectors[j].Priority()
	})
	return r
}

func DefaultRegistry() *Registry {
	return NewRegistry(
		NewClaudeDetector(),
		NewGeminiDetector(),
		NewGenericDetector(),
	)
}

func (r *Registry) Detect(paneContent string) (*PromptMatch, bool) {
	for _, d := range r.detectors {
		if match, ok := d.Detect(paneContent); ok {
			return match, true
		}
	}
	return nil, false
}
