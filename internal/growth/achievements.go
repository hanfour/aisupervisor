package growth

import "time"

type Achievement struct {
	ID          string    `yaml:"id" json:"id"`
	Name        string    `yaml:"name" json:"name"`
	NameZH      string    `yaml:"nameZh" json:"nameZh"`
	Description string    `yaml:"description" json:"description"`
	Icon        string    `yaml:"icon" json:"icon"`
	EXPReward   int       `yaml:"expReward" json:"expReward"`
	UnlockedAt  time.Time `yaml:"unlockedAt,omitempty" json:"unlockedAt,omitempty"`
}

type WorkerStats struct {
	TotalTasksCompleted  int
	ConsecutiveApprovals int
	TotalEXP             int
	MaxBranchLevel       int
	BranchesAtLevel3Plus int
	ReviewsCompleted     int
	FeedbacksReceived    int
	DaysActive           int
	TaskTypesCompleted   map[string]int
}

type achievementCheck func(stats WorkerStats) bool

type achievementDef struct {
	Achievement
	check achievementCheck
}

var defaultAchievements = []achievementDef{
	{Achievement{ID: "first_task", Name: "First Steps", NameZH: "初試啼聲", Description: "Complete your first task", Icon: "🎯", EXPReward: 50},
		func(s WorkerStats) bool { return s.TotalTasksCompleted >= 1 }},
	{Achievement{ID: "ten_tasks", Name: "Getting Started", NameZH: "漸入佳境", Description: "Complete 10 tasks", Icon: "📋", EXPReward: 100},
		func(s WorkerStats) bool { return s.TotalTasksCompleted >= 10 }},
	{Achievement{ID: "fifty_tasks", Name: "Veteran", NameZH: "資深老將", Description: "Complete 50 tasks", Icon: "🏆", EXPReward: 300},
		func(s WorkerStats) bool { return s.TotalTasksCompleted >= 50 }},
	{Achievement{ID: "perfect_streak_5", Name: "Flawless Five", NameZH: "五連完美", Description: "5 consecutive first-pass approvals", Icon: "⭐", EXPReward: 150},
		func(s WorkerStats) bool { return s.ConsecutiveApprovals >= 5 }},
	{Achievement{ID: "level_3", Name: "Skilled Worker", NameZH: "技能熟練", Description: "Reach level 3 in any branch", Icon: "📈", EXPReward: 200},
		func(s WorkerStats) bool { return s.MaxBranchLevel >= 3 }},
	{Achievement{ID: "level_5", Name: "Master", NameZH: "大師級", Description: "Reach level 5 in any branch", Icon: "👑", EXPReward: 500},
		func(s WorkerStats) bool { return s.MaxBranchLevel >= 5 }},
	{Achievement{ID: "polyglot", Name: "Polyglot", NameZH: "通才", Description: "Reach level 3+ in 3 or more branches", Icon: "🌟", EXPReward: 400},
		func(s WorkerStats) bool { return s.BranchesAtLevel3Plus >= 3 }},
	{Achievement{ID: "reviewer", Name: "Code Reviewer", NameZH: "程式碼審查員", Description: "Complete 10 reviews", Icon: "🔍", EXPReward: 150},
		func(s WorkerStats) bool { return s.ReviewsCompleted >= 10 }},
	{Achievement{ID: "mentor_badge", Name: "Mentor", NameZH: "導師", Description: "Receive 20 feedback interactions", Icon: "🎓", EXPReward: 200},
		func(s WorkerStats) bool { return s.FeedbacksReceived >= 20 }},
	{Achievement{ID: "versatile", Name: "Versatile", NameZH: "多面手", Description: "Complete tasks of 5+ different types", Icon: "🎭", EXPReward: 250},
		func(s WorkerStats) bool {
			return len(s.TaskTypesCompleted) >= 5
		}},
}

// AchievementEngine checks and tracks unlocked achievements per worker.
type AchievementEngine struct {
	unlocked map[string]map[string]Achievement // workerID -> achievementID -> Achievement
}

func NewAchievementEngine() *AchievementEngine {
	return &AchievementEngine{
		unlocked: make(map[string]map[string]Achievement),
	}
}

// SetUnlocked pre-loads unlocked achievements for a worker (from persistence).
func (ae *AchievementEngine) SetUnlocked(workerID string, achievements []Achievement) {
	ae.unlocked[workerID] = make(map[string]Achievement)
	for _, a := range achievements {
		ae.unlocked[workerID][a.ID] = a
	}
}

// Check evaluates all achievements for a worker and returns newly unlocked ones.
func (ae *AchievementEngine) Check(workerID string, stats WorkerStats) []Achievement {
	if ae.unlocked[workerID] == nil {
		ae.unlocked[workerID] = make(map[string]Achievement)
	}

	var newlyUnlocked []Achievement
	now := time.Now()
	for _, def := range defaultAchievements {
		if _, already := ae.unlocked[workerID][def.ID]; already {
			continue
		}
		if def.check(stats) {
			a := def.Achievement
			a.UnlockedAt = now
			ae.unlocked[workerID][a.ID] = a
			newlyUnlocked = append(newlyUnlocked, a)
		}
	}
	return newlyUnlocked
}

// GetAll returns all achievements with unlock status for a worker.
func (ae *AchievementEngine) GetAll(workerID string) []Achievement {
	unlocked := ae.unlocked[workerID]
	all := make([]Achievement, len(defaultAchievements))
	for i, def := range defaultAchievements {
		a := def.Achievement
		if u, ok := unlocked[a.ID]; ok {
			a.UnlockedAt = u.UnlockedAt
		}
		all[i] = a
	}
	return all
}

// GetUnlocked returns only unlocked achievements for a worker.
func (ae *AchievementEngine) GetUnlocked(workerID string) []Achievement {
	unlocked := ae.unlocked[workerID]
	var result []Achievement
	for _, a := range unlocked {
		result = append(result, a)
	}
	return result
}
