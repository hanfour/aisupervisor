package company

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/personality"
	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

type SimulationActivityType string

const (
	ActivityDiscussion  SimulationActivityType = "discussion"
	ActivityMeeting     SimulationActivityType = "meeting"
	ActivityWatercooler SimulationActivityType = "watercooler"
	ActivityCodeReview  SimulationActivityType = "code_review"
	ActivityTaskAssign  SimulationActivityType = "task_assign"
	ActivityThinking    SimulationActivityType = "thinking"
	ActivityWalking     SimulationActivityType = "walking"
)

type SimulationActivity struct {
	ID         string                 `json:"id"`
	Type       SimulationActivityType `json:"type"`
	WorkerIDs  []string               `json:"workerIds"`
	Message    string                 `json:"message"`
	Duration   int                    `json:"duration"`   // seconds
	ZoneTarget string                 `json:"zoneTarget"` // "meeting", "watercooler", "desk", etc.
	Priority   int                    `json:"priority"`
	CreatedAt  time.Time              `json:"createdAt"`
}

var thinkingMessages = []string{
	"Architecting the solution...",
	"Reading through the codebase...",
	"Debugging the issue...",
	"Writing tests first...",
	"Refactoring for clarity...",
	"Checking edge cases...",
	"Optimising the hot path...",
	"Deep in flow state...",
	"Untangling the dependency graph...",
	"Making the CI green...",
}

var watercoolerMessages = []string{
	"Coffee break ☕",
	"Taking a breather...",
	"Hydration station",
	"Grabbing a snack",
	"Quick mental reset",
	"Stretching legs",
}

var discussionMessages = []string{
	"Discussing API design",
	"Pair-programming session",
	"Quick sync on blockers",
	"Rubber-duck debugging",
	"Comparing notes on the task",
	"Talking through the architecture",
	"Brainstorming edge cases",
}

var meetingMessages = []string{
	"Sprint planning session",
	"Team stand-up",
	"Architecture review meeting",
	"Retrospective session",
	"All-hands sync",
	"Roadmap alignment meeting",
}

var codeReviewMessages = []string{
	"Reviewing PR — checking logic...",
	"Code review in progress",
	"Walking through the diff",
	"Leaving inline comments",
	"Verifying test coverage",
	"Checking for security issues",
}

var taskAssignMessages = []string{
	"Handing off the task brief",
	"Walking engineer through the spec",
	"Clarifying acceptance criteria",
	"Task kick-off chat",
	"Explaining the context",
}

func pick(rng *rand.Rand, items []string) string {
	return items[rng.Intn(len(items))]
}

// GenerateActivities inspects the current worker/task state and returns a
// slice of simulated office activities for the frontend to animate.
func (m *Manager) GenerateActivities() ([]SimulationActivity, error) {
	m.mu.RLock()
	workers := make([]*worker.Worker, 0, len(m.workers))
	for _, w := range m.workers {
		workers = append(workers, w)
	}
	m.mu.RUnlock()

	// Collect all tasks across all projects
	projects := m.projectStore.ListProjects()
	var allTasks []*project.Task
	for _, p := range projects {
		allTasks = append(allTasks, m.projectStore.TasksForProject(p.ID)...)
	}

	// Seed from current second so each tick is stable within a second but
	// changes every second — gives deterministic-ish variety.
	rng := rand.New(rand.NewSource(time.Now().Unix()))

	var activities []SimulationActivity

	var idleWorkers []*worker.Worker
	var workingWorkers []*worker.Worker

	for _, w := range workers {
		switch w.Status {
		case worker.WorkerWorking, worker.WorkerWaiting:
			workingWorkers = append(workingWorkers, w)
		case worker.WorkerIdle:
			idleWorkers = append(idleWorkers, w)
		}
	}

	// --- Working workers: 20% chance → thinking activity ---
	for _, w := range workingWorkers {
		if rng.Intn(100) < 20 {
			activities = append(activities, SimulationActivity{
				ID:         newActivityID(rng),
				Type:       ActivityThinking,
				WorkerIDs:  []string{w.ID},
				Message:    pick(rng, thinkingMessages),
				Duration:   10 + rng.Intn(20),
				ZoneTarget: "desk",
				Priority:   1,
				CreatedAt:  time.Now(),
			})
		}
	}

	// --- Idle workers: watercooler / discussion (personality-driven weights) ---
	for i, w := range idleWorkers {
		wcThreshold := 10
		discThreshold := 15
		if m.personalityStore != nil {
			profile := m.personalityStore.GetProfile(w.ID)
			if profile != nil {
				weights := personality.ComputeActivityWeights(profile)
				wcThreshold = weights.Watercooler
				discThreshold = wcThreshold + weights.Discussion
			}
		}

		roll := rng.Intn(100)
		switch {
		case roll < wcThreshold:
			// Watercooler trip
			activities = append(activities, SimulationActivity{
				ID:         newActivityID(rng),
				Type:       ActivityWatercooler,
				WorkerIDs:  []string{w.ID},
				Message:    pick(rng, watercoolerMessages),
				Duration:   15 + rng.Intn(30),
				ZoneTarget: "watercooler",
				Priority:   0,
				CreatedAt:  time.Now(),
			})
		case roll < discThreshold:
			// Walk to another idle worker for a discussion
			others := idleWorkers[:i]
			if len(others) == 0 {
				others = idleWorkers[i+1:]
			}
			if len(others) > 0 {
				partner := others[rng.Intn(len(others))]
				activities = append(activities, SimulationActivity{
					ID:         newActivityID(rng),
					Type:       ActivityDiscussion,
					WorkerIDs:  []string{w.ID, partner.ID},
					Message:    pick(rng, discussionMessages),
					Duration:   20 + rng.Intn(40),
					ZoneTarget: "discussion",
					Priority:   1,
					CreatedAt:  time.Now(),
				})
			}
		}
	}

	// --- 3+ idle workers: 5% chance → meeting ---
	if len(idleWorkers) >= 3 && rng.Intn(100) < 5 {
		ids := make([]string, len(idleWorkers))
		for i, w := range idleWorkers {
			ids[i] = w.ID
		}
		activities = append(activities, SimulationActivity{
			ID:         newActivityID(rng),
			Type:       ActivityMeeting,
			WorkerIDs:  ids,
			Message:    pick(rng, meetingMessages),
			Duration:   30 + rng.Intn(60),
			ZoneTarget: "meeting",
			Priority:   2,
			CreatedAt:  time.Now(),
		})
	}

	// --- Tasks in review → code_review activity ---
	for _, t := range allTasks {
		if t.Status != project.TaskReview {
			continue
		}
		// Find the engineer (assignee) and their manager
		var engineerID, managerID string
		if t.AssigneeID != "" {
			engineerID = t.AssigneeID
			m.mu.RLock()
			if eng, ok := m.workers[engineerID]; ok && eng.ParentID != "" {
				managerID = eng.ParentID
			}
			m.mu.RUnlock()
		}

		var workerIDs []string
		if managerID != "" {
			workerIDs = []string{managerID, engineerID}
		} else if engineerID != "" {
			workerIDs = []string{engineerID}
		}

		activities = append(activities, SimulationActivity{
			ID:        newActivityID(rng),
			Type:      ActivityCodeReview,
			WorkerIDs: workerIDs,
			Message:   fmt.Sprintf("%s — %s", pick(rng, codeReviewMessages), t.Title),
			Duration:  20 + rng.Intn(40),
			ZoneTarget: "meeting",
			Priority:  3,
			CreatedAt: time.Now(),
		})
		break // one code review activity per tick is enough
	}

	// --- Recently assigned tasks → task_assign activity ---
	for _, t := range allTasks {
		if t.Status != project.TaskAssigned {
			continue
		}
		if t.AssigneeID == "" {
			continue
		}
		// Only trigger if assigned within the last 2 minutes
		if time.Since(t.CreatedAt) > 2*time.Minute {
			continue
		}
		engineerID := t.AssigneeID
		var managerID string
		m.mu.RLock()
		if eng, ok := m.workers[engineerID]; ok && eng.ParentID != "" {
			managerID = eng.ParentID
		}
		m.mu.RUnlock()

		var workerIDs []string
		if managerID != "" {
			workerIDs = []string{managerID, engineerID}
		} else {
			workerIDs = []string{engineerID}
		}

		activities = append(activities, SimulationActivity{
			ID:         newActivityID(rng),
			Type:       ActivityTaskAssign,
			WorkerIDs:  workerIDs,
			Message:    fmt.Sprintf("%s: %s", pick(rng, taskAssignMessages), t.Title),
			Duration:   10 + rng.Intn(20),
			ZoneTarget: "desk",
			Priority:   2,
			CreatedAt:  time.Now(),
		})
		break // one per tick
	}

	return activities, nil
}

func newActivityID(rng *rand.Rand) string {
	return fmt.Sprintf("act-%d-%d", time.Now().UnixMilli(), rng.Int63n(1_000_000))
}
