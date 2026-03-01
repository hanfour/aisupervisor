package messaging

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/company"
	"github.com/hanfourmini/aisupervisor/internal/project"
)

// mockMessenger implements Messenger for testing.
type mockMessenger struct {
	mu            sync.Mutex
	notifications []string
	handler       CommandHandler
	started       bool
}

func newMockMessenger() *mockMessenger {
	return &mockMessenger{}
}

func (m *mockMessenger) Start(ctx context.Context) error {
	m.mu.Lock()
	m.started = true
	m.mu.Unlock()
	<-ctx.Done()
	return nil
}

func (m *mockMessenger) SendNotification(msg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifications = append(m.notifications, msg)
	return nil
}

func (m *mockMessenger) OnCommand(handler CommandHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handler = handler
}

func (m *mockMessenger) getNotifications() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.notifications))
	copy(result, m.notifications)
	return result
}

func (m *mockMessenger) isStarted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started
}

func (m *mockMessenger) simulateCommand(text string) string {
	m.mu.Lock()
	h := m.handler
	m.mu.Unlock()
	if h == nil {
		return ""
	}
	return h(text)
}

func testCompanyManager(t *testing.T) *company.Manager {
	t.Helper()
	dir := t.TempDir()
	store, err := project.NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	mgr, err := company.New(store, nil, nil, nil, nil, dir)
	if err != nil {
		t.Fatalf("company.New: %v", err)
	}
	return mgr
}

func TestNotifierStartsMessengers(t *testing.T) {
	mgr := testCompanyManager(t)
	m1 := newMockMessenger()
	m2 := newMockMessenger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	notifier := NewNotifier(mgr, []Messenger{m1, m2})
	notifier.Start(ctx)

	// Give goroutines time to start
	time.Sleep(50 * time.Millisecond)

	if !m1.isStarted() {
		t.Fatal("expected m1 to be started")
	}
	if !m2.isStarted() {
		t.Fatal("expected m2 to be started")
	}
}

func TestNotifierForwardsEvents(t *testing.T) {
	mgr := testCompanyManager(t)
	m1 := newMockMessenger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	notifier := NewNotifier(mgr, []Messenger{m1})
	notifier.Start(ctx)

	// Trigger a company event
	mgr.CreateProject("notify-test", "", "/tmp", "main", nil)

	// Wait for event to propagate
	time.Sleep(100 * time.Millisecond)

	notifications := m1.getNotifications()
	if len(notifications) == 0 {
		t.Fatal("expected at least one notification")
	}
	found := false
	for _, n := range notifications {
		if contains(n, "notify-test") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected notification containing 'notify-test', got: %v", notifications)
	}
}

func TestNotifierMultipleMessengers(t *testing.T) {
	mgr := testCompanyManager(t)
	m1 := newMockMessenger()
	m2 := newMockMessenger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	notifier := NewNotifier(mgr, []Messenger{m1, m2})
	notifier.Start(ctx)

	mgr.CreateWorker("TestBot", "robot")

	time.Sleep(100 * time.Millisecond)

	n1 := m1.getNotifications()
	n2 := m2.getNotifications()

	if len(n1) == 0 {
		t.Fatal("m1 should have received notification")
	}
	if len(n2) == 0 {
		t.Fatal("m2 should have received notification")
	}
}

func TestNotifierCommandHandler(t *testing.T) {
	mgr := testCompanyManager(t)
	m1 := newMockMessenger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	NewNotifier(mgr, []Messenger{m1})

	// Simulate incoming command
	reply := m1.simulateCommand("status")
	if !contains(reply, "Company Status") {
		t.Fatalf("expected status reply, got: %s", reply)
	}

	// Create a project via command
	reply = m1.simulateCommand("project create CommandProject")
	if !contains(reply, "Project created") {
		t.Fatalf("expected project creation, got: %s", reply)
	}

	// Verify project was actually created
	projects := mgr.ListProjects()
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	_ = ctx
}

func TestNotifierWorkerCommands(t *testing.T) {
	mgr := testCompanyManager(t)
	m1 := newMockMessenger()

	NewNotifier(mgr, []Messenger{m1})

	// Hire via command
	reply := m1.simulateCommand("worker hire Alice")
	if !contains(reply, "Worker hired") {
		t.Fatalf("expected hire reply, got: %s", reply)
	}

	// List via command
	reply = m1.simulateCommand("worker list")
	if !contains(reply, "Alice") {
		t.Fatalf("expected Alice in list, got: %s", reply)
	}
}

func TestNotifierHelpCommand(t *testing.T) {
	mgr := testCompanyManager(t)
	m1 := newMockMessenger()

	NewNotifier(mgr, []Messenger{m1})

	reply := m1.simulateCommand("help")
	if !contains(reply, "Available commands") {
		t.Fatalf("expected help text, got: %s", reply)
	}
}

// --- Event filter tests ---

func TestEventFilterEmpty(t *testing.T) {
	f := NewEventFilter(nil)
	if !f.Passes("anything") {
		t.Fatal("empty filter should pass all events")
	}
}

func TestEventFilterAllows(t *testing.T) {
	f := NewEventFilter([]string{"task_completed", "task_failed"})
	if !f.Passes("task_completed") {
		t.Fatal("should pass task_completed")
	}
	if !f.Passes("task_failed") {
		t.Fatal("should pass task_failed")
	}
	if f.Passes("project_created") {
		t.Fatal("should not pass project_created")
	}
}

func TestNotifierGlobalFilter(t *testing.T) {
	mgr := testCompanyManager(t)
	m1 := newMockMessenger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Only allow worker_spawned events
	notifier := NewNotifier(mgr, []Messenger{m1},
		WithGlobalFilter([]string{"worker_spawned"}))
	notifier.Start(ctx)

	// Create project (should be filtered out)
	mgr.CreateProject("filtered-proj", "", "/tmp", "main", nil)
	time.Sleep(100 * time.Millisecond)

	n := m1.getNotifications()
	for _, msg := range n {
		if contains(msg, "filtered-proj") {
			t.Fatalf("project_created should be filtered, got: %s", msg)
		}
	}

	// Create worker (should pass)
	mgr.CreateWorker("FilterBot", "robot")
	time.Sleep(100 * time.Millisecond)

	n = m1.getNotifications()
	found := false
	for _, msg := range n {
		if contains(msg, "FilterBot") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("worker_spawned should pass filter, got: %v", n)
	}
}

func TestNotifierPerMessengerFilter(t *testing.T) {
	mgr := testCompanyManager(t)
	m1 := newMockMessenger()
	m2 := newMockMessenger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	notifier := NewNotifier(mgr, []Messenger{m1, m2})
	// m1: only project events, m2: only worker events
	notifier.SetMessengerFilter(0, []string{"project_created"})
	notifier.SetMessengerFilter(1, []string{"worker_spawned"})
	notifier.Start(ctx)

	mgr.CreateProject("per-filter-proj", "", "/tmp", "main", nil)
	mgr.CreateWorker("PerFilterBot", "robot")
	time.Sleep(100 * time.Millisecond)

	// m1 should have project but not worker
	n1 := m1.getNotifications()
	hasProj := false
	hasWorker := false
	for _, msg := range n1 {
		if contains(msg, "per-filter-proj") {
			hasProj = true
		}
		if contains(msg, "PerFilterBot") {
			hasWorker = true
		}
	}
	if !hasProj {
		t.Fatalf("m1 should receive project event, got: %v", n1)
	}
	if hasWorker {
		t.Fatalf("m1 should NOT receive worker event, got: %v", n1)
	}

	// m2 should have worker but not project
	n2 := m2.getNotifications()
	hasProj = false
	hasWorker = false
	for _, msg := range n2 {
		if contains(msg, "per-filter-proj") {
			hasProj = true
		}
		if contains(msg, "PerFilterBot") {
			hasWorker = true
		}
	}
	if hasProj {
		t.Fatalf("m2 should NOT receive project event, got: %v", n2)
	}
	if !hasWorker {
		t.Fatalf("m2 should receive worker event, got: %v", n2)
	}
}

func TestNotifierPerMessengerOverridesGlobal(t *testing.T) {
	mgr := testCompanyManager(t)
	m1 := newMockMessenger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Global: only worker events. Per-messenger: only project events.
	// Per-messenger should win.
	notifier := NewNotifier(mgr, []Messenger{m1},
		WithGlobalFilter([]string{"worker_spawned"}))
	notifier.SetMessengerFilter(0, []string{"project_created"})
	notifier.Start(ctx)

	mgr.CreateProject("override-proj", "", "/tmp", "main", nil)
	mgr.CreateWorker("OverrideBot", "robot")
	time.Sleep(100 * time.Millisecond)

	n := m1.getNotifications()
	hasProj := false
	hasWorker := false
	for _, msg := range n {
		if contains(msg, "override-proj") {
			hasProj = true
		}
		if contains(msg, "OverrideBot") {
			hasWorker = true
		}
	}
	if !hasProj {
		t.Fatal("per-messenger filter should allow project_created")
	}
	if hasWorker {
		t.Fatal("per-messenger filter should block worker_spawned (not in its list)")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
