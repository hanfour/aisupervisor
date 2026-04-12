package growth

import "time"

type SkillBranch string

const (
	BranchFrontend     SkillBranch = "frontend"
	BranchBackend      SkillBranch = "backend"
	BranchSecurity     SkillBranch = "security"
	BranchDevOps       SkillBranch = "devops"
	BranchArchitecture SkillBranch = "architecture"
	BranchResearch     SkillBranch = "research"
)

var AllBranches = []SkillBranch{
	BranchFrontend, BranchBackend, BranchSecurity,
	BranchDevOps, BranchArchitecture, BranchResearch,
}

var LevelThresholds = map[int]int{
	1: 0,
	2: 100,
	3: 350,
	4: 800,
	5: 1500,
}

const MaxLevel = 5

type SkillNode struct {
	Branch     SkillBranch    `yaml:"branch" json:"branch"`
	Level      int            `yaml:"level" json:"level"`
	CurrentEXP int            `yaml:"currentExp" json:"currentExp"`
	TotalEXP   int            `yaml:"totalExp" json:"totalExp"`
	SubSkills  map[string]int `yaml:"subSkills" json:"subSkills"`
}

func (n *SkillNode) AddEXP(amount int) bool {
	if n.Level >= MaxLevel {
		n.TotalEXP += amount
		return false
	}
	n.CurrentEXP += amount
	n.TotalEXP += amount
	leveled := false
	for n.Level < MaxLevel {
		needed := LevelThresholds[n.Level+1] - LevelThresholds[n.Level]
		if n.CurrentEXP >= needed {
			n.CurrentEXP -= needed
			n.Level++
			leveled = true
		} else {
			break
		}
	}
	if n.Level >= MaxLevel {
		n.CurrentEXP = 0
	}
	return leveled
}

func (n *SkillNode) EXPToNextLevel() int {
	if n.Level >= MaxLevel {
		return 0
	}
	needed := LevelThresholds[n.Level+1] - LevelThresholds[n.Level]
	return needed - n.CurrentEXP
}

type SkillTree struct {
	Branches  map[SkillBranch]*SkillNode `yaml:"branches" json:"branches"`
	UpdatedAt time.Time                  `yaml:"updatedAt" json:"updatedAt"`
}

func NewSkillTree() *SkillTree {
	st := &SkillTree{
		Branches:  make(map[SkillBranch]*SkillNode),
		UpdatedAt: time.Now(),
	}
	for _, b := range AllBranches {
		st.Branches[b] = &SkillNode{
			Branch:    b,
			Level:     1,
			SubSkills: make(map[string]int),
		}
	}
	return st
}

func (st *SkillTree) DominantBranch() SkillBranch {
	var best SkillBranch
	var bestEXP int
	for _, node := range st.Branches {
		if node.TotalEXP > bestEXP {
			bestEXP = node.TotalEXP
			best = node.Branch
		}
	}
	return best
}

func (st *SkillTree) AverageLevel() float64 {
	total := 0
	for _, node := range st.Branches {
		total += node.Level
	}
	return float64(total) / float64(len(st.Branches))
}
