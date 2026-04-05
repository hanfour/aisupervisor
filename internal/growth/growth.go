// Package growth provides the employee growth system: skill trees, EXP, and feedback processing.
package growth

import (
	"strings"
	"time"
)

// SkillBranch represents a major skill category.
type SkillBranch string

const (
	BranchFrontend     SkillBranch = "frontend"
	BranchBackend      SkillBranch = "backend"
	BranchSecurity     SkillBranch = "security"
	BranchDevOps       SkillBranch = "devops"
	BranchArchitecture SkillBranch = "architecture"
	BranchResearch     SkillBranch = "research"
)

// AllBranches returns all skill branches.
func AllBranches() []SkillBranch {
	return []SkillBranch{
		BranchFrontend, BranchBackend, BranchSecurity,
		BranchDevOps, BranchArchitecture, BranchResearch,
	}
}

// SkillNode tracks progress in a single branch.
type SkillNode struct {
	Branch     SkillBranch        `yaml:"branch" json:"branch"`
	Level      int                `yaml:"level" json:"level"`
	CurrentEXP int               `yaml:"current_exp" json:"currentExp"`
	TotalEXP   int               `yaml:"total_exp" json:"totalExp"`
	SubSkills  map[string]int     `yaml:"sub_skills,omitempty" json:"subSkills,omitempty"`
}

// EXPToNextLevel returns the remaining EXP needed to reach the next level.
func (n *SkillNode) EXPToNextLevel() int {
	required := expForLevel(n.Level + 1)
	remaining := required - n.CurrentEXP
	if remaining < 0 {
		return 0
	}
	return remaining
}

// expForLevel returns the total EXP required for a given level.
func expForLevel(level int) int {
	if level <= 1 {
		return 0
	}
	// Exponential curve: 100, 300, 600, 1000, ...
	return level * level * 100
}

// SkillTree is the complete skill tree for a worker.
type SkillTree struct {
	Branches  map[SkillBranch]*SkillNode `yaml:"branches" json:"branches"`
	UpdatedAt time.Time                  `yaml:"updated_at" json:"updatedAt"`
}

// NewSkillTree creates a new skill tree with all branches at level 1.
func NewSkillTree() *SkillTree {
	branches := make(map[SkillBranch]*SkillNode)
	for _, b := range AllBranches() {
		branches[b] = &SkillNode{
			Branch:    b,
			Level:     1,
			SubSkills: make(map[string]int),
		}
	}
	return &SkillTree{
		Branches:  branches,
		UpdatedAt: time.Now(),
	}
}

// DominantBranch returns the branch with the highest level.
func (st *SkillTree) DominantBranch() SkillBranch {
	var best SkillBranch
	bestLevel := -1
	for b, n := range st.Branches {
		if n.Level > bestLevel {
			bestLevel = n.Level
			best = b
		}
	}
	return best
}

// AverageLevel returns the average level across all branches.
func (st *SkillTree) AverageLevel() float64 {
	if len(st.Branches) == 0 {
		return 0
	}
	total := 0
	for _, n := range st.Branches {
		total += n.Level
	}
	return float64(total) / float64(len(st.Branches))
}

// FeedbackType classifies boss feedback.
type FeedbackType string

const (
	FeedbackPositive FeedbackType = "positive"
	FeedbackNegative FeedbackType = "negative"
	FeedbackNeutral  FeedbackType = "neutral"
)

// FeedbackInfo carries processed feedback data.
type FeedbackInfo struct {
	Type    string      `json:"type"`
	Content string      `json:"content"`
	Branch  SkillBranch `json:"branch"`
}

// ClassifyFeedback determines if feedback is positive, negative, or neutral.
func ClassifyFeedback(content string) FeedbackType {
	lower := strings.ToLower(content)
	positiveWords := []string{"good", "great", "excellent", "well done", "nice", "amazing", "perfect", "awesome"}
	negativeWords := []string{"bad", "poor", "wrong", "fail", "terrible", "fix", "broken", "issue"}
	pos, neg := 0, 0
	for _, w := range positiveWords {
		if strings.Contains(lower, w) {
			pos++
		}
	}
	for _, w := range negativeWords {
		if strings.Contains(lower, w) {
			neg++
		}
	}
	if pos > neg {
		return FeedbackPositive
	}
	if neg > pos {
		return FeedbackNegative
	}
	return FeedbackNeutral
}

// InferBranches guesses which skill branches feedback relates to.
func InferBranches(content string) []SkillBranch {
	lower := strings.ToLower(content)
	var branches []SkillBranch
	keywords := map[SkillBranch][]string{
		BranchFrontend:     {"frontend", "ui", "css", "html", "svelte", "react", "vue"},
		BranchBackend:      {"backend", "api", "server", "database", "go", "rust", "python"},
		BranchSecurity:     {"security", "auth", "encrypt", "vulnerability", "xss", "injection"},
		BranchDevOps:       {"devops", "deploy", "ci", "cd", "docker", "kubernetes", "pipeline"},
		BranchArchitecture: {"architecture", "design", "pattern", "refactor", "structure"},
		BranchResearch:     {"research", "analysis", "investigate", "study", "explore"},
	}
	for branch, words := range keywords {
		for _, w := range words {
			if strings.Contains(lower, w) {
				branches = append(branches, branch)
				break
			}
		}
	}
	return branches
}

// Engine manages growth processing for all workers.
type Engine struct {
	trees map[string]*SkillTree // workerID -> SkillTree
}

// NewEngine creates a new growth engine.
func NewEngine() *Engine {
	return &Engine{
		trees: make(map[string]*SkillTree),
	}
}

// GetTree returns the skill tree for a worker, creating one if needed.
func (e *Engine) GetTree(workerID string) *SkillTree {
	if t, ok := e.trees[workerID]; ok {
		return t
	}
	t := NewSkillTree()
	e.trees[workerID] = t
	return t
}

// ProcessFeedback applies feedback to a worker's skill tree.
func (e *Engine) ProcessFeedback(workerID string, info FeedbackInfo) {
	tree := e.GetTree(workerID)
	branch := info.Branch
	if branch == "" {
		branch = BranchBackend // default
	}
	node, ok := tree.Branches[branch]
	if !ok {
		return
	}
	var expGain int
	switch FeedbackType(info.Type) {
	case FeedbackPositive:
		expGain = 50
	case FeedbackNegative:
		expGain = 10 // still learn from mistakes
	default:
		expGain = 25
	}
	node.CurrentEXP += expGain
	node.TotalEXP += expGain
	// Level up check
	for {
		required := expForLevel(node.Level + 1)
		if node.CurrentEXP >= required && node.Level < 5 {
			node.CurrentEXP -= required
			node.Level++
		} else {
			break
		}
	}
	tree.UpdatedAt = time.Now()
}
