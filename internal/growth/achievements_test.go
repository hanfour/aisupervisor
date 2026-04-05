package growth

import "testing"

func TestAchievementEngine_FirstTask(t *testing.T) {
	ae := NewAchievementEngine()
	stats := WorkerStats{TotalTasksCompleted: 1}
	newly := ae.Check("w1", stats)
	found := false
	for _, a := range newly {
		if a.ID == "first_task" {
			found = true
		}
	}
	if !found {
		t.Error("should unlock first_task achievement")
	}
}

func TestAchievementEngine_NoDuplicateUnlock(t *testing.T) {
	ae := NewAchievementEngine()
	stats := WorkerStats{TotalTasksCompleted: 1}
	ae.Check("w1", stats)
	newly := ae.Check("w1", stats)
	for _, a := range newly {
		if a.ID == "first_task" {
			t.Error("should not unlock first_task twice")
		}
	}
}

func TestAchievementEngine_MultipleUnlocks(t *testing.T) {
	ae := NewAchievementEngine()
	stats := WorkerStats{
		TotalTasksCompleted:  10,
		ConsecutiveApprovals: 5,
		MaxBranchLevel:       3,
	}
	newly := ae.Check("w1", stats)
	if len(newly) < 3 {
		t.Errorf("expected at least 3 unlocks (first_task, ten_tasks, perfect_streak_5), got %d", len(newly))
	}
}

func TestAchievementEngine_GetAll(t *testing.T) {
	ae := NewAchievementEngine()
	all := ae.GetAll("w1")
	if len(all) != 10 {
		t.Errorf("expected 10 achievements, got %d", len(all))
	}
	// All should be locked (zero UnlockedAt)
	for _, a := range all {
		if !a.UnlockedAt.IsZero() {
			t.Errorf("achievement %s should be locked", a.ID)
		}
	}
}

func TestAchievementEngine_Versatile(t *testing.T) {
	ae := NewAchievementEngine()
	stats := WorkerStats{
		TotalTasksCompleted: 5,
		TaskTypesCompleted: map[string]int{
			"code": 1, "research": 1, "prd": 1, "design": 1, "admin": 1,
		},
	}
	newly := ae.Check("w1", stats)
	found := false
	for _, a := range newly {
		if a.ID == "versatile" {
			found = true
		}
	}
	if !found {
		t.Error("should unlock versatile achievement")
	}
}
