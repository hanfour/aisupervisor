package worker

import (
	"testing"
)

func TestSpawnForTask_UsesWorktreePath(t *testing.T) {
	s := &Spawner{
		useWorktrees: true,
	}

	// Verify the worktree path format
	wtPath := s.worktreePath("/repo", "task-123")
	expected := "/repo/.worktrees/task-123"
	if wtPath != expected {
		t.Errorf("expected %s, got %s", expected, wtPath)
	}
}
