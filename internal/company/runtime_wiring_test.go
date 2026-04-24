package company

import (
	"testing"

	"github.com/hanfourmini/aisupervisor/internal/project"
)

// TestManagerNew_RuntimeRegistryAlwaysWired asserts that Manager.New() always
// constructs a non-nil RuntimeRegistry even when tmuxClient is nil, but skips
// plugin registration in that case so the registry exists but is empty.
//
// Guards I5 from PR #15 review: the registry-wiring block is otherwise
// untested and a future refactor could silently regress it to nil.
func TestManagerNew_RuntimeRegistryAlwaysWired(t *testing.T) {
	dir := t.TempDir()
	store, err := project.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Intentionally pass nil for tmuxClient, spawner, monitor, chatProvider.
	m, err := New(store, nil, nil, nil, nil, dir, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if m.runtimeRegistry == nil {
		t.Fatal("runtimeRegistry is nil after New() — expected a non-nil registry")
	}

	// With tmuxClient=nil, plugin registration must be skipped: list empty.
	names := m.runtimeRegistry.List()
	if len(names) != 0 {
		t.Fatalf("runtimeRegistry.List() with nil tmuxClient: got %v (len %d), want empty",
			names, len(names))
	}
	if d := m.runtimeRegistry.Default(); d != nil {
		t.Errorf("runtimeRegistry.Default() with nil tmuxClient: got %v, want nil", d)
	}
}
