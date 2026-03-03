package company

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/project"
)

func TestCheckBounceLoop_UnderLimit(t *testing.T) {
	cb := &CircuitBreaker{}
	task := &project.Task{ID: "t1", Title: "Test"}

	// No bounces yet
	if cb.CheckBounceLoop(task, "w1", "w2") {
		t.Error("should not detect loop with no bounces")
	}
}

func TestCheckBounceLoop_PairLimit(t *testing.T) {
	cb := &CircuitBreaker{}
	task := &project.Task{ID: "t1", Title: "Test"}

	// Add bounces between same pair
	for i := 0; i < MaxBouncesPerPair; i++ {
		task.BounceHistory = append(task.BounceHistory, project.BounceRecord{
			FromID: "w1",
			ToID:   "w2",
		})
	}

	if !cb.CheckBounceLoop(task, "w1", "w2") {
		t.Error("should detect loop at pair limit")
	}
}

func TestCheckBounceLoop_TotalLimit(t *testing.T) {
	cb := &CircuitBreaker{}
	task := &project.Task{ID: "t1", Title: "Test"}

	// Add bounces with different pairs
	for i := 0; i < MaxTotalBounces; i++ {
		task.BounceHistory = append(task.BounceHistory, project.BounceRecord{
			FromID: "w1",
			ToID:   "w" + string(rune('3'+i)),
		})
	}

	if !cb.CheckBounceLoop(task, "w1", "w99") {
		t.Error("should detect loop at total limit")
	}
}

func TestCheckBounceLoop_DifferentPair(t *testing.T) {
	cb := &CircuitBreaker{}
	task := &project.Task{ID: "t1", Title: "Test"}

	// Add bounces between w1-w2
	for i := 0; i < MaxBouncesPerPair; i++ {
		task.BounceHistory = append(task.BounceHistory, project.BounceRecord{
			FromID: "w1",
			ToID:   "w2",
		})
	}

	// Checking a different pair should not trigger (unless total exceeded)
	if MaxBouncesPerPair < MaxTotalBounces {
		if cb.CheckBounceLoop(task, "w3", "w4") {
			t.Error("should not detect loop for a different pair when under total limit")
		}
	}
}

func TestRecordBounce(t *testing.T) {
	cb := &CircuitBreaker{}
	task := &project.Task{ID: "t1"}

	cb.RecordBounce(task, "w1", "w2", project.TaskCodeReview, "style issues")

	if len(task.BounceHistory) != 1 {
		t.Fatalf("expected 1 bounce, got %d", len(task.BounceHistory))
	}
	if task.BounceHistory[0].FromID != "w1" {
		t.Error("wrong FromID")
	}
	if task.BounceHistory[0].Reason != "style issues" {
		t.Error("wrong Reason")
	}
}

func TestCheckBudget(t *testing.T) {
	cb := &CircuitBreaker{}

	// No budget limit
	task := &project.Task{BudgetLimit: 0, TokensConsumed: 999999}
	if cb.CheckBudget(task) {
		t.Error("should not trigger when no budget limit")
	}

	// Under budget
	task = &project.Task{BudgetLimit: 1000, TokensConsumed: 500}
	if cb.CheckBudget(task) {
		t.Error("should not trigger when under budget")
	}

	// Over budget
	task = &project.Task{BudgetLimit: 1000, TokensConsumed: 1000}
	if !cb.CheckBudget(task) {
		t.Error("should trigger when at budget limit")
	}
}

func TestBudgetWarning(t *testing.T) {
	cb := &CircuitBreaker{}

	// No budget
	task := &project.Task{BudgetLimit: 0}
	if cb.BudgetWarning(task) {
		t.Error("should not warn with no budget")
	}

	// Well within budget
	task = &project.Task{BudgetLimit: 1000, TokensConsumed: 500}
	if cb.BudgetWarning(task) {
		t.Error("should not warn at 50% budget")
	}

	// Within warning threshold (20%)
	task = &project.Task{BudgetLimit: 1000, TokensConsumed: 850}
	if !cb.BudgetWarning(task) {
		t.Error("should warn at 85% budget consumed")
	}
}
