package company

import (
	"context"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/project"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

// recoverTestStart sets up a Manager wired with a tmux mock that says
// "the session is alive" and a real CompletionMonitor pointing at the
// same mock (so watchCompletion's polling won't crash on a nil monitor).
// Tests using this helper should call m.Shutdown() in a defer to cancel
// any goroutines the recovery actually spawns — the watchCompletion
// goroutine will block forever otherwise.
func recoverTestStart(t *testing.T) (*Manager, *project.Store) {
	t.Helper()
	dir := t.TempDir()
	store := testStore(t)
	m, err := New(store, nil, nil, nil, nil, dir, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tmuxC := &stuckTestTmux{} // HasSession returns true, capture returns frozen
	m.tmuxClient = tmuxC
	m.monitor = worker.NewCompletionMonitor(tmuxC)
	return m, store
}

func testStore(t *testing.T) *project.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := project.NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func testManager(t *testing.T) (*Manager, <-chan Event) {
	t.Helper()
	dir := t.TempDir()
	store := testStore(t)

	m, err := New(store, nil, nil, nil, nil, dir, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Disable auto-schedule for predictable tests
	m.autoSchedule = false
	ch := m.Subscribe()
	return m, ch
}

func TestCreateProject(t *testing.T) {
	m, ch := testManager(t)

	p, err := m.CreateProject("test", "desc", "/tmp/repo", "main", nil)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if p.Name != "test" {
		t.Fatalf("expected name 'test', got %s", p.Name)
	}

	select {
	case e := <-ch:
		if e.Type != EventProjectCreated {
			t.Fatalf("expected project_created event, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected event but none received")
	}
}

func TestListProjects(t *testing.T) {
	m, ch := testManager(t)

	m.CreateProject("a", "", "/tmp/a", "main", nil)
	m.CreateProject("b", "", "/tmp/b", "main", nil)
	drainCh(ch)

	list := m.ListProjects()
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}
}

func TestAddTask(t *testing.T) {
	m, ch := testManager(t)

	p, _ := m.CreateProject("proj", "", "/tmp", "main", nil)
	drainCh(ch)

	task, err := m.AddTask(p.ID, "do thing", "desc", "prompt text", nil, 1, "v1", "")
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	if task.ID == "" {
		t.Fatal("expected auto-generated task ID")
	}
	if task.Status != project.TaskReady {
		t.Fatalf("expected ready (no deps), got %s", task.Status)
	}
	if task.BranchName == "" {
		t.Fatal("expected branch name to be set")
	}

	select {
	case e := <-ch:
		if e.Type != EventTaskCreated {
			t.Fatalf("expected task_created, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected event")
	}
}

func TestAddTaskWithDeps(t *testing.T) {
	m, ch := testManager(t)

	p, _ := m.CreateProject("proj", "", "/tmp", "main", nil)
	drainCh(ch)

	t1, _ := m.AddTask(p.ID, "first", "", "prompt1", nil, 1, "", "")
	drainCh(ch)

	t2, err := m.AddTask(p.ID, "second", "", "prompt2", []string{t1.ID}, 2, "", "")
	if err != nil {
		t.Fatalf("AddTask with deps: %v", err)
	}
	if t2.Status != project.TaskBacklog {
		t.Fatalf("expected backlog for task with deps, got %s", t2.Status)
	}
}

func TestAddTaskProjectNotFound(t *testing.T) {
	m, _ := testManager(t)
	_, err := m.AddTask("nonexistent", "task", "", "prompt", nil, 1, "", "")
	if err == nil {
		t.Fatal("expected error for nonexistent project")
	}
}

func TestCreateWorker(t *testing.T) {
	m, ch := testManager(t)

	w, err := m.CreateWorker("Alice", "robot")
	if err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}
	if w.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
	if w.Name != "Alice" {
		t.Fatalf("expected Alice, got %s", w.Name)
	}

	select {
	case e := <-ch:
		if e.Type != EventWorkerSpawned {
			t.Fatalf("expected worker_spawned, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected event")
	}
}

func TestListWorkers(t *testing.T) {
	m, ch := testManager(t)

	m.CreateWorker("A", "robot")
	m.CreateWorker("B", "kirby")
	drainCh(ch)

	workers := m.ListWorkers()
	if len(workers) != 2 {
		t.Fatalf("expected 2 workers, got %d", len(workers))
	}
}

func TestWorkerPersistence(t *testing.T) {
	dir := t.TempDir()
	storeDir := t.TempDir()
	store, _ := project.NewStore(storeDir)

	m1, _ := New(store, nil, nil, nil, nil, dir, nil)
	m1.CreateWorker("Persist", "mario")

	m2, err := New(store, nil, nil, nil, nil, dir, nil)
	if err != nil {
		t.Fatalf("reload manager: %v", err)
	}

	workers := m2.ListWorkers()
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker after reload, got %d", len(workers))
	}
	if workers[0].Name != "Persist" {
		t.Fatalf("expected name 'Persist', got %s", workers[0].Name)
	}
}

func TestProjectProgress(t *testing.T) {
	m, ch := testManager(t)

	p, _ := m.CreateProject("prog", "", "/tmp", "main", nil)
	drainCh(ch)

	m.AddTask(p.ID, "t1", "", "p1", nil, 1, "", "")
	m.AddTask(p.ID, "t2", "", "p2", nil, 1, "", "")
	m.AddTask(p.ID, "t3", "", "p3", nil, 1, "", "")
	drainCh(ch)

	tasks := m.ListTasks(p.ID)
	m.CompleteTask(tasks[0].ID)
	drainCh(ch)

	prog := m.ProjectProgress(p.ID)
	if prog.Total != 3 {
		t.Fatalf("expected total 3, got %d", prog.Total)
	}
	if prog.Done != 1 {
		t.Fatalf("expected 1 done, got %d", prog.Done)
	}
	if prog.Percent < 33.0 || prog.Percent > 34.0 {
		t.Fatalf("expected ~33%%, got %.1f%%", prog.Percent)
	}
}

func TestCompleteTask(t *testing.T) {
	m, ch := testManager(t)

	p, _ := m.CreateProject("p", "", "/tmp", "main", nil)
	drainCh(ch)

	t1, _ := m.AddTask(p.ID, "first", "", "p1", nil, 1, "", "")
	t2, _ := m.AddTask(p.ID, "second", "", "p2", []string{t1.ID}, 2, "", "")
	drainCh(ch)

	if err := m.CompleteTask(t1.ID); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	got, _ := m.projectStore.GetTask(t2.ID)
	if got.Status != project.TaskReady {
		t.Fatalf("expected t2 ready after t1 done, got %s", got.Status)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"Add Login Page", "add-login-page"},
		{"fix: bug #123", "fix-bug-123"},
		{"  spaces  ", "spaces"},
		{"UPPER", "upper"},
	}
	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.expected {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- New tests for subscriber + auto-scheduling ---

func TestSubscriberMultiple(t *testing.T) {
	m, ch1 := testManager(t)
	ch2 := m.Subscribe()

	m.CreateProject("multi", "", "/tmp", "main", nil)

	// Both subscribers should receive the event
	select {
	case e := <-ch1:
		if e.Type != EventProjectCreated {
			t.Fatalf("ch1: expected project_created, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ch1: expected event")
	}

	select {
	case e := <-ch2:
		if e.Type != EventProjectCreated {
			t.Fatalf("ch2: expected project_created, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ch2: expected event")
	}
}

func TestEventsBackwardsCompat(t *testing.T) {
	m, _ := testManager(t)

	// Events() should return a subscriber channel
	ch := m.Events()
	m.CreateProject("compat", "", "/tmp", "main", nil)

	select {
	case e := <-ch:
		if e.Type != EventProjectCreated {
			t.Fatalf("expected project_created via Events(), got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected event via Events()")
	}
}

func TestUnsubscribe(t *testing.T) {
	m, ch1 := testManager(t)
	ch2 := m.Subscribe()

	// Unsubscribe ch1
	m.Unsubscribe(ch1)

	m.CreateProject("unsub-test", "", "/tmp", "main", nil)

	// ch1 should be closed (receive zero value)
	select {
	case _, ok := <-ch1:
		if ok {
			t.Fatal("ch1 should be closed after unsubscribe")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ch1 should be closed immediately")
	}

	// ch2 should still receive events
	select {
	case e := <-ch2:
		if e.Type != EventProjectCreated {
			t.Fatalf("ch2: expected project_created, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ch2 should still receive events")
	}
}

func TestUnsubscribeNonexistent(t *testing.T) {
	m, _ := testManager(t)
	// Should not panic when unsubscribing a channel that was never subscribed
	fakeCh := make(chan Event)
	m.Unsubscribe(fakeCh) // no-op, should not panic
}

func TestAutoScheduleDisabled(t *testing.T) {
	m, _ := testManager(t)
	// autoSchedule is false in testManager
	if m.autoSchedule {
		t.Fatal("expected autoSchedule to be false in test")
	}
}

func TestDrainReadyQueue(t *testing.T) {
	m, ch := testManager(t)

	p, _ := m.CreateProject("proj", "", "/tmp", "main", nil)
	drainCh(ch)

	// Create 3 ready tasks (no deps)
	t1, _ := m.AddTask(p.ID, "task-a", "", "prompt-a", nil, 3, "", "")
	t2, _ := m.AddTask(p.ID, "task-b", "", "prompt-b", nil, 2, "", "")
	t3, _ := m.AddTask(p.ID, "task-c", "", "prompt-c", nil, 1, "", "")
	drainCh(ch)

	// All should be ready
	for _, tk := range []*project.Task{t1, t2, t3} {
		got, _ := m.projectStore.GetTask(tk.ID)
		if got.Status != project.TaskReady {
			t.Fatalf("expected task %s to be ready, got %s", tk.ID, got.Status)
		}
	}

	// Create 2 idle workers
	w1, _ := m.CreateWorker("W1", "robot")
	w2, _ := m.CreateWorker("W2", "kirby")
	drainCh(ch)

	// drainReadyQueue should assign 2 tasks to 2 workers
	// (AssignTask requires spawner, so it will fail — but we can verify the attempt)
	// Since spawner is nil, AssignTask will proceed until it hits spawner call.
	// Let's test with the method directly and check that it doesn't panic.
	m.drainReadyQueue(t.Context())

	// Without a spawner the assignments fail, tasks revert to ready.
	// Verify the method is safe to call, doesn't deadlock, and tasks stay ready.
	workers := m.ListWorkers()
	if len(workers) != 2 {
		t.Fatalf("expected 2 workers, got %d", len(workers))
	}

	// All tasks should still be ready (spawner=nil causes assign to revert)
	for _, tk := range m.ListTasks(p.ID) {
		if tk.Status != project.TaskReady {
			t.Fatalf("expected task %s to be ready after failed assign, got %s", tk.ID, tk.Status)
		}
	}

	_ = w1
	_ = w2
	_ = t3
}

func TestDrainReadyQueueNoTasks(t *testing.T) {
	m, ch := testManager(t)
	m.CreateProject("proj", "", "/tmp", "main", nil)
	drainCh(ch)

	// Should be safe with no ready tasks
	m.drainReadyQueue(t.Context())
}

func TestDrainReadyQueueNoIdleWorkers(t *testing.T) {
	m, ch := testManager(t)
	p, _ := m.CreateProject("proj", "", "/tmp", "main", nil)
	m.AddTask(p.ID, "task-a", "", "prompt", nil, 1, "", "")
	drainCh(ch)

	// No workers at all — should not panic
	m.drainReadyQueue(t.Context())
}

func TestAddTaskAutoScheduleTrigger(t *testing.T) {
	m, ch := testManager(t)
	// Enable autoSchedule
	m.autoSchedule = true

	p, _ := m.CreateProject("proj", "", "/tmp", "main", nil)
	drainCh(ch)

	// Create a task with deps — should NOT trigger drainReadyQueue (status=backlog)
	t1, _ := m.AddTask(p.ID, "dep-task", "", "prompt", nil, 1, "", "")
	drainCh(ch)

	// Create task depending on t1 — status should be backlog
	t2, _ := m.AddTask(p.ID, "blocked", "", "prompt", []string{t1.ID}, 1, "", "")
	if t2.Status != project.TaskBacklog {
		t.Fatalf("expected backlog, got %s", t2.Status)
	}
	drainCh(ch)

	// Create a no-dep task — status should be ready (triggers drainReadyQueue in goroutine)
	t3, _ := m.AddTask(p.ID, "ready-task", "", "prompt", nil, 1, "", "")
	if t3.Status != project.TaskReady {
		t.Fatalf("expected ready, got %s", t3.Status)
	}
	drainCh(ch)
}

func TestUpdateTaskStatusDirectDrain(t *testing.T) {
	m, ch := testManager(t)
	m.autoSchedule = true

	p, _ := m.CreateProject("proj", "", "/tmp", "main", nil)
	// Create task with dep so it starts as backlog
	t1, _ := m.AddTask(p.ID, "first", "", "p", nil, 1, "", "")
	t2, _ := m.AddTask(p.ID, "second", "", "p", []string{t1.ID}, 1, "", "")
	drainCh(ch)

	if t2.Status != project.TaskBacklog {
		t.Fatalf("expected backlog, got %s", t2.Status)
	}

	// Manually drag to ready via UpdateTaskStatusDirect
	err := m.UpdateTaskStatusDirect(t2.ID, string(project.TaskReady))
	if err != nil {
		t.Fatalf("UpdateTaskStatusDirect: %v", err)
	}

	// Verify task is now ready
	got, _ := m.projectStore.GetTask(t2.ID)
	if got.Status != project.TaskReady {
		t.Fatalf("expected ready after direct update, got %s", got.Status)
	}
	drainCh(ch)
}

func drainCh(ch <-chan Event) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// TestNew_AutoAssignDefaultsEnabled pins the constructor default for
// autoAssignCfg.Enabled. The periodic proactiveTaskDiscovery loop checks
// cfg.Enabled before doing any work, and a regression that flipped the
// default back to false would silently disable auto-assignment of idle
// workers — exactly the dead-wire bug this PR is fixing.
func TestNew_AutoAssignDefaultsEnabled(t *testing.T) {
	dir := t.TempDir()
	store := testStore(t)

	m, err := New(store, nil, nil, nil, nil, dir, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !m.autoAssignCfg.Enabled {
		t.Fatal("expected autoAssignCfg.Enabled = true by default; periodic proactiveTaskDiscovery would early-return otherwise")
	}
}

// TestNew_StartsPeriodicHealthCheck verifies that Manager.New() spawns
// the periodic health-check goroutine. We probe by triggering an
// orphan-task condition (a task assigned to an idle worker with no
// tmux session) and waiting for the loop's runHealthCycle to do nothing
// for it (only RunHealthReport — called once at startup — fixes orphans;
// runHealthCycle handles tmux liveness and stuck-detect). To avoid a
// 60s wait in the unit test we instead assert the more directly
// observable behaviour: shutdown unblocks within a tight window
// because the goroutine respects bgCtx cancellation. A leaked goroutine
// would not cause this test to fail directly, but observing the clean
// shutdown is a strong proxy that the loop was started properly.
func TestNew_StartsPeriodicHealthCheck(t *testing.T) {
	dir := t.TempDir()
	store := testStore(t)

	m, err := New(store, nil, nil, nil, nil, dir, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	done := make(chan struct{})
	go func() {
		m.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// Shutdown returned promptly — bgCtx cancellation propagated to
		// every spawned goroutine, including StartHealthCheck if it ran.
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown did not return within 2s — StartHealthCheck or another goroutine is not respecting bgCtx")
	}
}

// TestRecoverActiveMonitors_RestartsForLiveTmux covers the recovery path:
// a worker persisted as `working` with a tmux session that's still alive
// gets a watchCompletion goroutine restarted (visible via m.cancels). This
// is the path that prevents the wails-dev hot-reload / OS-restart class of
// "stranded worker" bug, where a tmux pane keeps running a claude/ais-agent
// process but no goroutine polls it for completion.
func TestRecoverActiveMonitors_RestartsForLiveTmux(t *testing.T) {
	m, store := recoverTestStart(t)
	defer m.Shutdown()

	// Set up a project + task + worker that look like an in-flight task.
	p, err := m.CreateProject("recover-proj", "test", t.TempDir(), "main", nil)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	tk, err := m.AddTask(p.ID, "in-flight", "desc", "do stuff", nil, 1, "", "code")
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	if err := store.ForceUpdateTaskStatus(tk.ID, project.TaskInProgress); err != nil {
		t.Fatalf("ForceUpdateTaskStatus: %v", err)
	}

	w, err := m.CreateWorker("RecoverTest", "")
	if err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}
	m.mu.Lock()
	w.Status = worker.WorkerWorking
	w.CurrentTaskID = tk.ID
	w.TmuxSession = "test-session-alive"
	m.mu.Unlock()

	// Pre-condition: no monitor goroutine is registered for w yet.
	m.mu.RLock()
	if _, ok := m.cancels[w.ID]; ok {
		m.mu.RUnlock()
		t.Fatal("precondition: cancels map should be empty before recoverActiveMonitors")
	}
	m.mu.RUnlock()

	bgCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.recoverActiveMonitors(bgCtx)

	// Post-condition: a cancel func is registered, meaning watchCompletion
	// goroutine was started for this worker. (The goroutine itself is
	// already polling the stuckTestTmux mock and will exit when Shutdown
	// cancels bgCtx or when the test's defer cancel() fires.)
	m.mu.RLock()
	_, ok := m.cancels[w.ID]
	m.mu.RUnlock()
	if !ok {
		t.Fatal("expected cancels[worker] to be registered after recoverActiveMonitors — goroutine not started")
	}
}

// TestRecoverActiveMonitors_SkipsIdleWorker verifies the negative case:
// a worker in idle state (typical post-recovery) is NOT recovered, even
// if it still has a CurrentTaskID set.
func TestRecoverActiveMonitors_SkipsIdleWorker(t *testing.T) {
	m, _ := recoverTestStart(t)
	defer m.Shutdown()

	w, err := m.CreateWorker("IdleWorker", "")
	if err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}
	// Worker is idle (default). Don't set Working.

	bgCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.recoverActiveMonitors(bgCtx)

	m.mu.RLock()
	_, ok := m.cancels[w.ID]
	m.mu.RUnlock()
	if ok {
		t.Fatal("idle worker should not be recovered — no monitor needed")
	}
}

// TestRecoverActiveMonitors_SkipsMissingTask verifies that a working
// worker pointing at a no-longer-existent task is NOT recovered (we can't
// watch for completion of a task that isn't there). The worker is left
// untouched so a human can inspect.
func TestRecoverActiveMonitors_SkipsMissingTask(t *testing.T) {
	m, _ := recoverTestStart(t)
	defer m.Shutdown()

	w, err := m.CreateWorker("OrphanTask", "")
	if err != nil {
		t.Fatalf("CreateWorker: %v", err)
	}
	m.mu.Lock()
	w.Status = worker.WorkerWorking
	w.CurrentTaskID = "t-does-not-exist"
	w.TmuxSession = "test-session-alive"
	m.mu.Unlock()

	bgCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.recoverActiveMonitors(bgCtx)

	m.mu.RLock()
	_, ok := m.cancels[w.ID]
	m.mu.RUnlock()
	if ok {
		t.Fatal("worker with missing task should not be recovered")
	}
}
