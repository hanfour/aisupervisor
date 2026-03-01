package group

// Opinion represents a single role's evaluation result during the opinion phase.
type Opinion struct {
	RoleID     string
	RoleName   string
	Action     string // the chosen option key or action
	Reasoning  string
	Confidence float64
}

// DivergenceResult captures whether opinions diverge significantly.
type DivergenceResult struct {
	Divergent      bool
	UniqueActions  []string
	ConfidenceSpread float64
}

// DetectDivergence checks whether the given opinions diverge beyond the threshold.
// Divergence is detected when:
// 1. There are multiple unique actions (different chosen options), OR
// 2. The confidence spread (max - min) exceeds the threshold.
func DetectDivergence(opinions []Opinion, threshold float64) DivergenceResult {
	if len(opinions) <= 1 {
		return DivergenceResult{}
	}

	// Collect unique actions
	actionSet := make(map[string]bool)
	for _, op := range opinions {
		actionSet[op.Action] = true
	}
	unique := make([]string, 0, len(actionSet))
	for a := range actionSet {
		unique = append(unique, a)
	}

	// Calculate confidence spread
	minConf := opinions[0].Confidence
	maxConf := opinions[0].Confidence
	for _, op := range opinions[1:] {
		if op.Confidence < minConf {
			minConf = op.Confidence
		}
		if op.Confidence > maxConf {
			maxConf = op.Confidence
		}
	}
	spread := maxConf - minConf

	divergent := len(unique) > 1 || spread > threshold

	return DivergenceResult{
		Divergent:        divergent,
		UniqueActions:    unique,
		ConfidenceSpread: spread,
	}
}
