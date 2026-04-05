package growth

import (
	"fmt"
	"sync"
	"time"
)

type TaskCompletedInfo struct {
	TaskType               string
	Difficulty             float64
	ReviewPassedFirstTime  bool
	ReviewAttempts         int
	TokenEfficiency        float64
	VerifyCmdPassed        bool
	BugCount               int
	ConsecutiveCompletions int
	CompletionTime         time.Duration
}

type FeedbackInfo struct {
	Type    string
	Content string
	Branch  SkillBranch
}

type Engine struct {
	mu         sync.RWMutex
	skillTrees map[string]*SkillTree
	streaks    map[string]int
}

func NewEngine() *Engine {
	return &Engine{
		skillTrees: make(map[string]*SkillTree),
		streaks:    make(map[string]int),
	}
}

func (e *Engine) SetSkillTree(workerID string, st *SkillTree) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.skillTrees[workerID] = st
}

func (e *Engine) GetSkillTree(workerID string) *SkillTree {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.skillTrees[workerID]
}

func (e *Engine) ProcessTaskCompleted(workerID string, info TaskCompletedInfo) []GrowthEvent {
	e.mu.Lock()
	defer e.mu.Unlock()

	st, ok := e.skillTrees[workerID]
	if !ok {
		return nil
	}

	quality := EvaluateQuality(QualitySignals{
		ReviewPassedFirstTime: info.ReviewPassedFirstTime,
		ReviewAttempts:        info.ReviewAttempts,
		TokenEfficiency:       info.TokenEfficiency,
		VerifyCmdPassed:       info.VerifyCmdPassed,
		BugCount:              info.BugCount,
		CompletionTime:        info.CompletionTime,
	})

	difficulty := info.Difficulty
	if difficulty == 0 {
		difficulty = 1.0
	}

	calc := EXPCalculation{
		BaseEXP:        BaseEXPForTaskType(info.TaskType),
		DifficultyMult: difficulty,
		QualityMult:    QualityToMult(quality),
		ReviewBonus:    ReviewBonusMult(info.ReviewPassedFirstTime, info.ReviewAttempts),
		StreakBonus:     StreakBonusMult(info.ConsecutiveCompletions),
	}

	totalEXP := calc.Total()
	branches := BranchesForTaskType(info.TaskType)
	now := time.Now()

	var events []GrowthEvent

	for _, bw := range branches {
		node, exists := st.Branches[bw.Branch]
		if !exists {
			continue
		}
		amount := int(float64(totalEXP) * bw.Weight)
		if amount < 1 {
			amount = 1
		}
		prevLevel := node.Level
		leveled := node.AddEXP(amount)

		events = append(events, GrowthEvent{
			Type:      GrowthEXPGained,
			WorkerID:  workerID,
			Branch:    bw.Branch,
			Amount:    amount,
			Message:   fmt.Sprintf("+%d EXP (%s)", amount, bw.Branch),
			Timestamp: now,
		})

		if leveled {
			events = append(events, GrowthEvent{
				Type:      GrowthLevelUp,
				WorkerID:  workerID,
				Branch:    bw.Branch,
				NewLevel:  node.Level,
				Message:   fmt.Sprintf("%s: Lv.%d → Lv.%d", bw.Branch, prevLevel, node.Level),
				Timestamp: now,
			})
		}
	}

	st.UpdatedAt = now
	return events
}

func (e *Engine) ProcessFeedback(workerID string, info FeedbackInfo) []GrowthEvent {
	e.mu.Lock()
	defer e.mu.Unlock()

	st, ok := e.skillTrees[workerID]
	if !ok {
		return nil
	}

	now := time.Now()
	var events []GrowthEvent

	var bonus int
	switch info.Type {
	case "praise":
		bonus = 20
	case "correction":
		bonus = 5
	default:
		bonus = 0
	}

	if bonus > 0 && info.Branch != "" {
		node, exists := st.Branches[info.Branch]
		if exists {
			prevLevel := node.Level
			leveled := node.AddEXP(bonus)
			events = append(events, GrowthEvent{
				Type:      GrowthFeedback,
				WorkerID:  workerID,
				Branch:    info.Branch,
				Amount:    bonus,
				Message:   fmt.Sprintf("Boss feedback (%s): +%d EXP", info.Type, bonus),
				Timestamp: now,
			})
			if leveled {
				events = append(events, GrowthEvent{
					Type:      GrowthLevelUp,
					WorkerID:  workerID,
					Branch:    info.Branch,
					NewLevel:  node.Level,
					Message:   fmt.Sprintf("%s: Lv.%d → Lv.%d (from feedback)", info.Branch, prevLevel, node.Level),
					Timestamp: now,
				})
			}
		}
	}

	st.UpdatedAt = now
	return events
}
