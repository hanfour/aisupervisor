package personality

import (
	"strings"
	"testing"
)

func TestNewDefaultSkillScores(t *testing.T) {
	scores := NewDefaultSkillScores()
	if scores.Carefulness != 50 {
		t.Errorf("Carefulness = %d, want 50", scores.Carefulness)
	}
	if scores.CodeQuality != 50 {
		t.Errorf("CodeQuality = %d, want 50", scores.CodeQuality)
	}
}

func TestApplySkillEvent_ReviewRejectedBugs(t *testing.T) {
	scores := NewDefaultSkillScores()
	ApplySkillEvent(&scores, SkillEventReviewRejectedBugs)

	if scores.Carefulness != 42 { // 50 - 8
		t.Errorf("Carefulness = %d, want 42", scores.Carefulness)
	}
	if scores.BoundaryChecking != 45 { // 50 - 5
		t.Errorf("BoundaryChecking = %d, want 45", scores.BoundaryChecking)
	}
	// Other scores unchanged
	if scores.CodeQuality != 50 {
		t.Errorf("CodeQuality = %d, want 50", scores.CodeQuality)
	}
}

func TestApplySkillEvent_ReviewApproved(t *testing.T) {
	scores := NewDefaultSkillScores()
	ApplySkillEvent(&scores, SkillEventReviewApproved)

	if scores.Carefulness != 53 { // 50 + 3
		t.Errorf("Carefulness = %d, want 53", scores.Carefulness)
	}
	if scores.CodeQuality != 55 { // 50 + 5
		t.Errorf("CodeQuality = %d, want 55", scores.CodeQuality)
	}
}

func TestApplySkillEvent_Clamping(t *testing.T) {
	scores := SkillScores{Carefulness: 5}
	ApplySkillEvent(&scores, SkillEventReviewRejectedBugs) // -8

	if scores.Carefulness != 0 {
		t.Errorf("Carefulness = %d, want 0 (clamped)", scores.Carefulness)
	}

	scores = SkillScores{CodeQuality: 98}
	ApplySkillEvent(&scores, SkillEventReviewApproved) // +5

	if scores.CodeQuality != 100 {
		t.Errorf("CodeQuality = %d, want 100 (clamped)", scores.CodeQuality)
	}
}

func TestDecayTowardBaseline(t *testing.T) {
	scores := SkillScores{
		Carefulness:   80,
		CodeQuality:   20,
		SecurityAwareness: 50,
	}

	DecayTowardBaseline(&scores)

	// 80 → 80 - (80-50)/10 = 80 - 3 = 77
	if scores.Carefulness != 77 {
		t.Errorf("Carefulness = %d, want 77", scores.Carefulness)
	}
	// 20 → 20 - (20-50)/10 = 20 - (-3) = 23
	if scores.CodeQuality != 23 {
		t.Errorf("CodeQuality = %d, want 23", scores.CodeQuality)
	}
	// 50 → stays 50
	if scores.SecurityAwareness != 50 {
		t.Errorf("SecurityAwareness = %d, want 50", scores.SecurityAwareness)
	}
}

func TestGenerateSkillPrompt_NoIssues(t *testing.T) {
	scores := NewDefaultSkillScores() // all 50
	prompt := GenerateSkillPrompt(scores)
	if prompt != "" {
		t.Error("expected empty prompt for neutral scores")
	}
}

func TestGenerateSkillPrompt_LowScore(t *testing.T) {
	scores := NewDefaultSkillScores()
	scores.Carefulness = 30

	prompt := GenerateSkillPrompt(scores)
	if !strings.Contains(prompt, "Carefulness") {
		t.Error("expected Carefulness warning in prompt")
	}
	if !strings.Contains(prompt, "weak") {
		t.Error("expected 'weak' keyword in prompt")
	}
}

func TestGenerateSkillPrompt_HighScore(t *testing.T) {
	scores := NewDefaultSkillScores()
	scores.CodeQuality = 90

	prompt := GenerateSkillPrompt(scores)
	if !strings.Contains(prompt, "Code Quality") {
		t.Error("expected Code Quality mention in prompt")
	}
	if !strings.Contains(prompt, "strong") {
		t.Error("expected 'strong' keyword in prompt")
	}
}

func TestClassifyRejectionType(t *testing.T) {
	cases := map[string]SkillEventType{
		"missing test coverage":        SkillEventReviewRejectedTests,
		"SQL injection vulnerability":  SkillEventReviewRejectedSecurity,
		"nil pointer error":            SkillEventReviewRejectedBugs,
		"code style issues":            SkillEventReviewRejectedStyle,
		"authentication bypass":        SkillEventReviewRejectedSecurity,
		"race condition detected":      SkillEventReviewRejectedBugs,
		"needs better naming":          SkillEventReviewRejectedStyle,
	}

	for feedback, expected := range cases {
		got := ClassifyRejectionType(feedback)
		if got != expected {
			t.Errorf("ClassifyRejectionType(%q) = %s, want %s", feedback, got, expected)
		}
	}
}

func TestApplySkillEvent_UnknownEvent(t *testing.T) {
	scores := NewDefaultSkillScores()
	original := scores
	ApplySkillEvent(&scores, "unknown_event")
	if scores != original {
		t.Error("unknown event should not change scores")
	}
}
