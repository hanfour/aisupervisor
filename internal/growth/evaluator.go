package growth

import (
	"math"
	"time"
)

type EXPCalculation struct {
	BaseEXP        int
	DifficultyMult float64
	QualityMult    float64
	ReviewBonus    float64
	FeedbackBonus  int
	StreakBonus     float64
}

func (c EXPCalculation) Total() int {
	base := float64(c.BaseEXP)
	mult := c.DifficultyMult * c.QualityMult * c.ReviewBonus * c.StreakBonus
	return int(math.Round(base*mult)) + c.FeedbackBonus
}

type QualitySignals struct {
	ReviewPassedFirstTime bool
	ReviewAttempts        int
	TokenEfficiency       float64
	VerifyCmdPassed       bool
	BugCount              int
	CompletionTime        time.Duration
}

func EvaluateQuality(s QualitySignals) float64 {
	score := 0.5

	if s.ReviewPassedFirstTime {
		score += 0.2
	} else {
		penalty := float64(s.ReviewAttempts-1) * 0.1
		if penalty > 0.3 {
			penalty = 0.3
		}
		score -= penalty
	}

	score += s.TokenEfficiency * 0.15

	if s.VerifyCmdPassed {
		score += 0.1
	} else {
		score -= 0.1
	}

	bugPenalty := float64(s.BugCount) * 0.05
	if bugPenalty > 0.2 {
		bugPenalty = 0.2
	}
	score -= bugPenalty

	if score < 0.0 {
		score = 0.0
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

func QualityToMult(quality float64) float64 {
	return 0.5 + quality*1.5
}

func ReviewBonusMult(passedFirstTime bool, attempts int) float64 {
	if passedFirstTime {
		return 1.5
	}
	if attempts <= 2 {
		return 1.0
	}
	return 0.8
}

func StreakBonusMult(consecutiveCompletions int) float64 {
	switch {
	case consecutiveCompletions >= 5:
		return 1.2
	case consecutiveCompletions >= 3:
		return 1.1
	default:
		return 1.0
	}
}

func BaseEXPForTaskType(taskType string) int {
	switch taskType {
	case "code":
		return 50
	case "research":
		return 40
	case "prd":
		return 35
	case "design":
		return 45
	case "admin":
		return 20
	case "hr":
		return 25
	case "training":
		return 60
	default:
		return 30
	}
}

type BranchWeight struct {
	Branch SkillBranch
	Weight float64
}

func BranchesForTaskType(taskType string) []BranchWeight {
	switch taskType {
	case "code":
		return []BranchWeight{{BranchBackend, 0.6}, {BranchFrontend, 0.4}}
	case "design":
		return []BranchWeight{{BranchFrontend, 0.8}, {BranchArchitecture, 0.2}}
	case "research":
		return []BranchWeight{{BranchResearch, 0.7}, {BranchArchitecture, 0.3}}
	case "prd":
		return []BranchWeight{{BranchArchitecture, 0.5}, {BranchResearch, 0.5}}
	case "training":
		return []BranchWeight{{BranchBackend, 0.5}, {BranchFrontend, 0.5}}
	case "admin":
		return []BranchWeight{{BranchDevOps, 0.7}, {BranchArchitecture, 0.3}}
	case "hr":
		return []BranchWeight{{BranchResearch, 0.8}, {BranchArchitecture, 0.2}}
	default:
		return []BranchWeight{{BranchBackend, 1.0}}
	}
}
