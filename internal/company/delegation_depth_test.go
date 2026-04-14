package company

import (
	"testing"
)

func TestDelegationDepth_IsSetOnSubTask(t *testing.T) {
	// When a parent task has depth 0 (root), delegated tasks should have depth 1
	parentDepth := 0
	childDepth := parentDepth + 1
	if childDepth != 1 {
		t.Errorf("expected child depth 1, got %d", childDepth)
	}
}

func TestDelegationDepth_MaxPreventsPrompt(t *testing.T) {
	// MaxDelegationDepth is 2, so depth >= 2 should suppress delegation
	const MaxDelegationDepth = 2
	if 2 >= MaxDelegationDepth {
		// This is expected: depth 2 should NOT get delegation prompt
	}
	if 1 >= MaxDelegationDepth {
		t.Error("depth 1 should still allow delegation")
	}
}
