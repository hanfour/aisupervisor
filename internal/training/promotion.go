package training

import (
	"fmt"
	"time"
)

// PromotionCriteria defines the requirements for tier promotion.
type PromotionCriteria struct {
	MinTrainingPairs    int     `yaml:"min_training_pairs" json:"min_training_pairs"`
	MinBenchmarkScore   float64 `yaml:"min_benchmark_score" json:"min_benchmark_score"`
	ConsecutivePasses   int     `yaml:"consecutive_passes" json:"consecutive_passes"`
	MinApprovalRate     float64 `yaml:"min_approval_rate" json:"min_approval_rate"` // 0.0 - 1.0
}

// DefaultPromotionCriteria returns reasonable defaults.
func DefaultPromotionCriteria() PromotionCriteria {
	return PromotionCriteria{
		MinTrainingPairs:  100,
		MinBenchmarkScore: 0.8,
		ConsecutivePasses: 3,
		MinApprovalRate:   0.85,
	}
}

// PromotionRecord tracks a worker's promotion history.
type PromotionRecord struct {
	WorkerID      string    `json:"worker_id"`
	FromTier      string    `json:"from_tier"`
	ToTier        string    `json:"to_tier"`
	ModelVersion  string    `json:"model_version"`
	BenchmarkScore float64  `json:"benchmark_score"`
	ApprovalRate  float64   `json:"approval_rate"`
	PromotedAt    time.Time `json:"promoted_at"`
}

// PromotionChecker evaluates whether a worker qualifies for tier promotion.
type PromotionChecker struct {
	criteria PromotionCriteria
	registry *ModelRegistry
}

func NewPromotionChecker(criteria PromotionCriteria, registry *ModelRegistry) *PromotionChecker {
	return &PromotionChecker{
		criteria: criteria,
		registry: registry,
	}
}

// PromotionStatus holds the current state toward promotion.
type PromotionStatus struct {
	Eligible          bool    `json:"eligible"`
	TrainingPairs     int     `json:"training_pairs"`
	LatestBenchmark   float64 `json:"latest_benchmark"`
	ConsecutivePasses int     `json:"consecutive_passes"`
	ApprovalRate      float64 `json:"approval_rate"`
	Reasons           []string `json:"reasons,omitempty"` // why not eligible
}

// Check evaluates if the given model version meets promotion criteria.
func (pc *PromotionChecker) Check(modelVersion string, evalRuns []EvalRun, approvalRate float64, trainingPairs int) PromotionStatus {
	status := PromotionStatus{
		TrainingPairs: trainingPairs,
		ApprovalRate:  approvalRate,
	}

	// Check minimum training pairs
	if trainingPairs < pc.criteria.MinTrainingPairs {
		status.Reasons = append(status.Reasons,
			fmt.Sprintf("need %d training pairs, have %d", pc.criteria.MinTrainingPairs, trainingPairs))
	}

	// Check approval rate
	if approvalRate < pc.criteria.MinApprovalRate {
		status.Reasons = append(status.Reasons,
			fmt.Sprintf("need %.0f%% approval rate, have %.0f%%", pc.criteria.MinApprovalRate*100, approvalRate*100))
	}

	// Check consecutive benchmark passes
	consecutivePasses := 0
	var latestScore float64
	for i := len(evalRuns) - 1; i >= 0; i-- {
		run := evalRuns[i]
		if run.ModelVer != modelVersion {
			continue
		}
		if i == len(evalRuns)-1 {
			latestScore = run.AvgScore
		}
		if run.AvgScore >= pc.criteria.MinBenchmarkScore {
			consecutivePasses++
		} else {
			break
		}
	}
	status.LatestBenchmark = latestScore
	status.ConsecutivePasses = consecutivePasses

	if consecutivePasses < pc.criteria.ConsecutivePasses {
		status.Reasons = append(status.Reasons,
			fmt.Sprintf("need %d consecutive benchmark passes, have %d", pc.criteria.ConsecutivePasses, consecutivePasses))
	}

	if latestScore < pc.criteria.MinBenchmarkScore {
		status.Reasons = append(status.Reasons,
			fmt.Sprintf("need %.0f%% benchmark score, have %.0f%%", pc.criteria.MinBenchmarkScore*100, latestScore*100))
	}

	status.Eligible = len(status.Reasons) == 0
	return status
}
