package growth

import (
	"testing"
	"time"
)

func TestCalculateEXP_BasicTask(t *testing.T) {
	calc := EXPCalculation{
		BaseEXP:        50,
		DifficultyMult: 1.0,
		QualityMult:    1.0,
		ReviewBonus:    1.0,
		FeedbackBonus:  0,
		StreakBonus:     1.0,
	}
	exp := calc.Total()
	if exp != 50 {
		t.Errorf("expected 50 EXP, got %d", exp)
	}
}

func TestCalculateEXP_WithBonuses(t *testing.T) {
	calc := EXPCalculation{
		BaseEXP:        50,
		DifficultyMult: 2.0,
		QualityMult:    1.5,
		ReviewBonus:    1.5,
		FeedbackBonus:  20,
		StreakBonus:     1.1,
	}
	exp := calc.Total()
	if exp != 268 {
		t.Errorf("expected 268 EXP, got %d", exp)
	}
}

func TestQualityScore(t *testing.T) {
	signals := QualitySignals{
		ReviewPassedFirstTime: true,
		ReviewAttempts:        1,
		TokenEfficiency:       0.8,
		VerifyCmdPassed:       true,
		BugCount:              0,
		CompletionTime:        30 * time.Minute,
	}
	score := EvaluateQuality(signals)
	if score < 0.8 {
		t.Errorf("high quality signals should score > 0.8, got %.2f", score)
	}
}

func TestQualityScore_Poor(t *testing.T) {
	signals := QualitySignals{
		ReviewPassedFirstTime: false,
		ReviewAttempts:        3,
		TokenEfficiency:       0.3,
		VerifyCmdPassed:       false,
		BugCount:              5,
		CompletionTime:        4 * time.Hour,
	}
	score := EvaluateQuality(signals)
	if score > 0.5 {
		t.Errorf("poor quality signals should score < 0.5, got %.2f", score)
	}
}
