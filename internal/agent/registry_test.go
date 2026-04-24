package agent

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// fakeRuntime is a minimal AgentRuntime implementation used only in tests.
// All methods return zero values; the registry tests only care about
// identity (Name) and registration/lookup semantics.
type fakeRuntime struct {
	name string
}

func (f *fakeRuntime) Name() string { return f.name }

func (f *fakeRuntime) Spawn(ctx context.Context, cfg SpawnConfig) (*AgentSession, error) {
	return nil, nil
}

func (f *fakeRuntime) SendPrompt(session *AgentSession, prompt string) error { return nil }

func (f *fakeRuntime) CaptureOutput(session *AgentSession, lines int) (string, error) {
	return "", nil
}

func (f *fakeRuntime) DetectReady(ctx context.Context, session *AgentSession, timeout time.Duration) error {
	return nil
}

func (f *fakeRuntime) DetectCompletion(ctx context.Context, session *AgentSession, content string) (bool, error) {
	return false, nil
}

func (f *fakeRuntime) ParseTokenUsage(output string) (TokenUsage, error) {
	return TokenUsage{}, nil
}

func (f *fakeRuntime) Cleanup(session *AgentSession) error { return nil }

func (f *fakeRuntime) MonitoredSessionType() string { return "fake" }

func TestRuntimeRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRuntimeRegistry()
	rt := &fakeRuntime{name: "claudecode"}
	reg.Register(rt)

	got, ok := reg.Get("claudecode")
	if !ok {
		t.Fatalf("Get(claudecode) returned ok=false, want true")
	}
	if got != rt {
		t.Fatalf("Get(claudecode) returned a different instance; want %p got %p", rt, got)
	}
}

func TestRuntimeRegistry_GetUnknown(t *testing.T) {
	reg := NewRuntimeRegistry()
	reg.Register(&fakeRuntime{name: "claudecode"})

	got, ok := reg.Get("does-not-exist")
	if ok {
		t.Fatalf("Get(unknown) returned ok=true, want false")
	}
	if got != nil {
		t.Fatalf("Get(unknown) returned non-nil runtime %v, want nil", got)
	}
}

func TestRuntimeRegistry_Default(t *testing.T) {
	// Empty registry: Default() must return nil.
	empty := NewRuntimeRegistry()
	if def := empty.Default(); def != nil {
		t.Fatalf("empty registry Default() = %v, want nil", def)
	}

	// First registered is the default.
	reg := NewRuntimeRegistry()
	first := &fakeRuntime{name: "claudecode"}
	second := &fakeRuntime{name: "aisagent"}
	third := &fakeRuntime{name: "aider"}
	reg.Register(first)
	reg.Register(second)
	reg.Register(third)

	if got := reg.Default(); got != first {
		t.Fatalf("Default() = %v, want first registered %v", got, first)
	}
}

func TestRuntimeRegistry_List(t *testing.T) {
	reg := NewRuntimeRegistry()
	reg.Register(&fakeRuntime{name: "claudecode"})
	reg.Register(&fakeRuntime{name: "aisagent"})
	reg.Register(&fakeRuntime{name: "aider"})

	got := reg.List()
	want := []string{"claudecode", "aisagent", "aider"}

	if len(got) != len(want) {
		t.Fatalf("List() length = %d, want %d (got=%v)", len(got), len(want), got)
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("List()[%d] = %q, want %q (full=%v)", i, got[i], name, got)
		}
	}

	// Re-registering an existing name must not duplicate the entry and must
	// preserve the original insertion order.
	reg.Register(&fakeRuntime{name: "aisagent"})
	got = reg.List()
	if len(got) != len(want) {
		t.Fatalf("after re-register, List() length = %d, want %d (got=%v)", len(got), len(want), got)
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("after re-register, List()[%d] = %q, want %q (full=%v)", i, got[i], name, got)
		}
	}
}

func TestRuntimeRegistry_ConcurrentAccess(t *testing.T) {
	reg := NewRuntimeRegistry()

	const workers = 16
	const opsPerWorker = 200

	var wg sync.WaitGroup
	wg.Add(workers * 2)

	// Writers: register runtimes with varying names.
	for w := 0; w < workers; w++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerWorker; i++ {
				name := fmt.Sprintf("rt-%d-%d", id, i%4)
				reg.Register(&fakeRuntime{name: name})
			}
		}(w)
	}

	// Readers: call Get/List/Default concurrently.
	for w := 0; w < workers; w++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerWorker; i++ {
				_, _ = reg.Get(fmt.Sprintf("rt-%d-%d", id, i%4))
				_ = reg.List()
				_ = reg.Default()
			}
		}(w)
	}

	wg.Wait()
}
