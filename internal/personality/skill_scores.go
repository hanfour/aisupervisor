package personality

import (
	"fmt"
	"strings"
)

// SkillScores represents work quality dimensions for a worker.
type SkillScores struct {
	Carefulness          int `yaml:"carefulness" json:"carefulness"`
	BoundaryChecking     int `yaml:"boundary_checking" json:"boundaryChecking"`
	TestCoverageAware    int `yaml:"test_coverage_aware" json:"testCoverageAware"`
	CommunicationClarity int `yaml:"communication_clarity" json:"communicationClarity"`
	CodeQuality          int `yaml:"code_quality" json:"codeQuality"`
	SecurityAwareness    int `yaml:"security_awareness" json:"securityAwareness"`
}

// NewDefaultSkillScores returns skill scores with all values at 50 (neutral).
func NewDefaultSkillScores() SkillScores {
	return SkillScores{
		Carefulness:          50,
		BoundaryChecking:     50,
		TestCoverageAware:    50,
		CommunicationClarity: 50,
		CodeQuality:          50,
		SecurityAwareness:    50,
	}
}

// SkillEventType categorizes review feedback for skill score adjustment.
type SkillEventType string

const (
	SkillEventReviewRejectedBugs     SkillEventType = "review_rejected_bugs"
	SkillEventReviewRejectedStyle    SkillEventType = "review_rejected_style"
	SkillEventReviewRejectedSecurity SkillEventType = "review_rejected_security"
	SkillEventReviewRejectedTests    SkillEventType = "review_rejected_tests"
	SkillEventReviewApproved         SkillEventType = "review_approved"
	SkillEventTestsFailed            SkillEventType = "tests_failed"
	SkillEventTestsPassed            SkillEventType = "tests_passed"
	SkillEventSecurityFailed         SkillEventType = "security_failed"
	SkillEventSecurityPassed         SkillEventType = "security_passed"
)

type skillDelta struct {
	Carefulness          int
	BoundaryChecking     int
	TestCoverageAware    int
	CommunicationClarity int
	CodeQuality          int
	SecurityAwareness    int
}

var skillRules = map[SkillEventType]skillDelta{
	SkillEventReviewRejectedBugs:     {Carefulness: -8, BoundaryChecking: -5},
	SkillEventReviewRejectedStyle:    {CodeQuality: -5, CommunicationClarity: -3},
	SkillEventReviewRejectedSecurity: {SecurityAwareness: -8, Carefulness: -3},
	SkillEventReviewRejectedTests:    {TestCoverageAware: -8, Carefulness: -5},
	SkillEventReviewApproved:         {Carefulness: 3, CodeQuality: 5},
	SkillEventTestsFailed:            {TestCoverageAware: -8, Carefulness: -5},
	SkillEventTestsPassed:            {TestCoverageAware: 3},
	SkillEventSecurityFailed:         {SecurityAwareness: -8},
	SkillEventSecurityPassed:         {SecurityAwareness: 3},
}

// ApplySkillEvent adjusts skill scores based on an event type.
func ApplySkillEvent(scores *SkillScores, event SkillEventType) {
	delta, ok := skillRules[event]
	if !ok {
		return
	}
	scores.Carefulness = clamp(scores.Carefulness+delta.Carefulness, 0, 100)
	scores.BoundaryChecking = clamp(scores.BoundaryChecking+delta.BoundaryChecking, 0, 100)
	scores.TestCoverageAware = clamp(scores.TestCoverageAware+delta.TestCoverageAware, 0, 100)
	scores.CommunicationClarity = clamp(scores.CommunicationClarity+delta.CommunicationClarity, 0, 100)
	scores.CodeQuality = clamp(scores.CodeQuality+delta.CodeQuality, 0, 100)
	scores.SecurityAwareness = clamp(scores.SecurityAwareness+delta.SecurityAwareness, 0, 100)
}

// DecayTowardBaseline moves all scores 10% closer to 50 (the neutral baseline).
// Should be called every 10 completed tasks.
func DecayTowardBaseline(scores *SkillScores) {
	decay := func(v int) int {
		diff := v - 50
		return v - diff/10
	}
	scores.Carefulness = decay(scores.Carefulness)
	scores.BoundaryChecking = decay(scores.BoundaryChecking)
	scores.TestCoverageAware = decay(scores.TestCoverageAware)
	scores.CommunicationClarity = decay(scores.CommunicationClarity)
	scores.CodeQuality = decay(scores.CodeQuality)
	scores.SecurityAwareness = decay(scores.SecurityAwareness)
}

// GenerateSkillPrompt produces dynamic prompt guidance based on current skill scores.
// Scores below 40 generate warnings; scores above 80 generate positive reinforcement.
func GenerateSkillPrompt(scores SkillScores) string {
	var warnings []string
	var strengths []string

	check := func(name string, value int) {
		if value < 40 {
			warnings = append(warnings, fmt.Sprintf("⚠ %s is weak (score: %d). Pay extra attention to this area.", name, value))
		} else if value > 80 {
			strengths = append(strengths, fmt.Sprintf("✓ %s is strong (score: %d). Keep up the good work.", name, value))
		}
	}

	check("Carefulness", scores.Carefulness)
	check("Boundary Checking", scores.BoundaryChecking)
	check("Test Coverage Awareness", scores.TestCoverageAware)
	check("Communication Clarity", scores.CommunicationClarity)
	check("Code Quality", scores.CodeQuality)
	check("Security Awareness", scores.SecurityAwareness)

	if len(warnings) == 0 && len(strengths) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n--- Skill Profile ---\n")
	for _, w := range warnings {
		sb.WriteString(w + "\n")
	}
	for _, s := range strengths {
		sb.WriteString(s + "\n")
	}
	return sb.String()
}

// ClassifyRejectionType determines the skill event type from review feedback text.
func ClassifyRejectionType(feedback string) SkillEventType {
	lower := strings.ToLower(feedback)

	securityKeywords := []string{"security", "vulnerability", "injection", "xss", "csrf", "auth", "permission", "exploit"}
	testKeywords := []string{"test", "coverage", "untested", "missing test", "no test"}
	bugKeywords := []string{"bug", "error", "crash", "nil pointer", "panic", "race condition", "incorrect", "wrong"}

	for _, kw := range securityKeywords {
		if strings.Contains(lower, kw) {
			return SkillEventReviewRejectedSecurity
		}
	}
	for _, kw := range testKeywords {
		if strings.Contains(lower, kw) {
			return SkillEventReviewRejectedTests
		}
	}
	for _, kw := range bugKeywords {
		if strings.Contains(lower, kw) {
			return SkillEventReviewRejectedBugs
		}
	}

	return SkillEventReviewRejectedStyle
}
