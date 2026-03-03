package verify

import "time"

// StepName identifies a verification step.
type StepName string

const (
	StepLint     StepName = "lint"
	StepBuild    StepName = "build"
	StepTest     StepName = "test"
	StepSecurity StepName = "security"
)

// StepResult holds the outcome of a single verification step.
type StepResult struct {
	Step     StepName      `json:"step"`
	Passed   bool          `json:"passed"`
	Output   string        `json:"output"`
	Duration time.Duration `json:"duration"`
}

// VerificationResult holds the aggregate outcome of all verification steps.
type VerificationResult struct {
	Passed  bool           `json:"passed"`
	Steps   []StepResult   `json:"steps"`
	Summary string         `json:"summary"`
}

// FirstFailure returns the first failed step, or nil if all passed.
func (vr *VerificationResult) FirstFailure() *StepResult {
	for i := range vr.Steps {
		if !vr.Steps[i].Passed {
			return &vr.Steps[i]
		}
	}
	return nil
}
