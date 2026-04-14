package worker

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/project"
)

func TestShouldIncludeDelegation_RootTask(t *testing.T) {
	task := &project.Task{DelegationDepth: 0}
	if !shouldIncludeDelegation(task) {
		t.Error("root task (depth 0) should include delegation prompt")
	}
}

func TestShouldIncludeDelegation_ChildTask(t *testing.T) {
	task := &project.Task{DelegationDepth: 1}
	if !shouldIncludeDelegation(task) {
		t.Error("depth 1 task should still include delegation prompt")
	}
}

func TestShouldIncludeDelegation_MaxDepth(t *testing.T) {
	task := &project.Task{DelegationDepth: 2}
	if shouldIncludeDelegation(task) {
		t.Error("depth 2 task should NOT include delegation prompt")
	}
}
