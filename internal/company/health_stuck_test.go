package company

import (
	"context"
	"testing"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/tmux"
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

// stuckTestTmux is a minimal tmux.TmuxClient mock for stuck-detect tests.
// Returns sessionExists=true and the same pane content on every capture so
// the stuckness heuristic CAN trigger when un-monitored.
type stuckTestTmux struct {
	sentKeysCalls int
	killCalls     int
}

func (s *stuckTestTmux) ListSessions() ([]tmux.SessionInfo, error) {
	return nil, nil
}
func (s *stuckTestTmux) ListPanes(session string) ([]tmux.PaneInfo, error) {
	return nil, nil
}
func (s *stuckTestTmux) CapturePane(session string, window, pane, lines int) (string, error) {
	return "frozen content\n", nil
}
func (s *stuckTestTmux) SendKeys(session string, window, pane int, keys string) error {
	s.sentKeysCalls++
	return nil
}
func (s *stuckTestTmux) SendLiteralKeys(session string, window, pane int, text string) error {
	return nil
}
func (s *stuckTestTmux) CreateSession(name string) error { return nil }
func (s *stuckTestTmux) KillSession(name string) error {
	s.killCalls++
	return nil
}
func (s *stuckTestTmux) HasSession(name string) (bool, error) { return true, nil }

// withShortStuckThreshold temporarily lowers the stuck threshold so tests
// don't have to wait 30 minutes.
func withShortStuckThreshold(t *testing.T, d time.Duration) func() {
	t.Helper()
	orig := stuckThreshold
	stuckThreshold = d
	return func() { stuckThreshold = orig }
}

// TestRunHealthCycle_SkipsMonitoredWorker verifies the new "skip when
// m.cancels[w.ID] is set" guard in runHealthCycle: a worker with an
// active monitor must NOT be touched by HEALTH's stuck-detect path,
// even when its pane content has been frozen past stuckThreshold.
func TestRunHealthCycle_SkipsMonitoredWorker(t *testing.T) {
	defer withShortStuckThreshold(t, 100*time.Millisecond)()

	m, _ := testManager(t)
	tmuxC := &stuckTestTmux{}
	m.tmuxClient = tmuxC

	w := &worker.Worker{
		ID:          "w-monitored",
		Name:        "Monitored",
		Status:      worker.WorkerWorking,
		TmuxSession: "test-session",
	}
	m.workers[w.ID] = w

	// Pre-seed lastPaneContent so since is past stuckThreshold immediately.
	m.lastPaneContent[w.ID] = paneSnapshot{
		content: "frozen content\n",
		since:   time.Now().Add(-1 * time.Second),
	}

	// Mark worker as monitored — the new guard should fire here.
	_, cancel := context.WithCancel(context.Background())
	m.cancels[w.ID] = cancel
	defer cancel()

	m.runHealthCycle()

	// Stuck-detect path was skipped → no SendKeys, no KillSession,
	// no RecoveryAttempts increment.
	if tmuxC.sentKeysCalls != 0 {
		t.Errorf("monitored worker received SendKeys (%d calls) — stuck-detect should have been skipped",
			tmuxC.sentKeysCalls)
	}
	if tmuxC.killCalls != 0 {
		t.Errorf("monitored worker session was killed — stuck-detect should have been skipped")
	}
	if w.RecoveryAttempts != 0 {
		t.Errorf("RecoveryAttempts = %d, want 0 (stuck path not taken)", w.RecoveryAttempts)
	}
}

// TestRunHealthCycle_StuckUnmonitoredWorker verifies stuck-detect still
// fires for workers WITHOUT a monitor (legacy/recovery paths). Confirms
// the new guard didn't accidentally disable the safety net.
func TestRunHealthCycle_StuckUnmonitoredWorker(t *testing.T) {
	defer withShortStuckThreshold(t, 100*time.Millisecond)()

	m, _ := testManager(t)
	tmuxC := &stuckTestTmux{}
	m.tmuxClient = tmuxC

	w := &worker.Worker{
		ID:          "w-unmonitored",
		Name:        "Unmonitored",
		Status:      worker.WorkerWorking,
		TmuxSession: "test-session",
	}
	m.workers[w.ID] = w

	// Pre-seed lastPaneContent so since is past stuckThreshold immediately.
	m.lastPaneContent[w.ID] = paneSnapshot{
		content: "frozen content\n",
		since:   time.Now().Add(-1 * time.Second),
	}
	// Note: NOT putting w.ID into m.cancels — worker is un-monitored.

	m.runHealthCycle()

	// First stuck attempt sends Enter (no kill yet).
	if tmuxC.sentKeysCalls != 1 {
		t.Errorf("expected 1 SendKeys (Enter) for stuck unmonitored worker, got %d", tmuxC.sentKeysCalls)
	}
	if w.RecoveryAttempts != 1 {
		t.Errorf("RecoveryAttempts = %d, want 1", w.RecoveryAttempts)
	}
}
