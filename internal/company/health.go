package company

import (
	"context"
	"log"
	"time"

	"github.com/hanfourmini/aisupervisor/internal/worker"
)

// paneSnapshot records captured pane content and when it was first seen unchanged.
type paneSnapshot struct {
	content string
	since   time.Time
}

// StartHealthCheck runs a periodic background health check every 60 seconds.
// It detects orphaned tmux sessions and stalled workers.
func (m *Manager) StartHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.runHealthCycle()
		}
	}
}

// runHealthCycle performs a single health check iteration.
func (m *Manager) runHealthCycle() {
	if m.tmuxClient == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, w := range m.workers {
		if w.Status != worker.WorkerWorking && w.Status != worker.WorkerWaiting {
			continue
		}
		if w.TmuxSession == "" {
			continue
		}

		// Check if tmux session still exists
		has, err := m.tmuxClient.HasSession(w.TmuxSession)
		if err != nil || !has {
			log.Printf("HEALTH: worker %s (%s) has orphaned session %q — resetting", w.ID, w.Name, w.TmuxSession)

			// Cancel completion monitor
			if cancel, ok := m.cancels[w.ID]; ok {
				cancel()
				delete(m.cancels, w.ID)
			}

			w.Status = worker.WorkerIdle
			w.CurrentTaskID = ""
			w.TmuxSession = ""
			w.SessionID = ""
			m.saveWorkers()

			m.emit(Event{
				Type:     EventHumanInterventionRequired,
				WorkerID: w.ID,
				Message:  m.msgf("Worker %s auto-reset: orphaned tmux session", "員工 %s 已自動重設：孤立的 tmux session", w.Name),
			})

			// Clean up stale pane snapshot
			delete(m.lastPaneContent, w.ID)
			continue
		}

		// Capture pane content and check for stalled output
		content, err := m.tmuxClient.CapturePane(w.TmuxSession, 0, 0, 20)
		if err != nil {
			continue
		}

		prev, exists := m.lastPaneContent[w.ID]
		if !exists || prev.content != content {
			// Content changed — update snapshot
			m.lastPaneContent[w.ID] = paneSnapshot{content: content, since: time.Now()}
		} else if time.Since(prev.since) > 5*time.Minute {
			// Content unchanged for 5+ minutes — emit warning (don't auto-reset)
			log.Printf("HEALTH: worker %s (%s) pane output unchanged for %v", w.ID, w.Name, time.Since(prev.since).Round(time.Second))
			m.emit(Event{
				Type:     EventHumanInterventionRequired,
				WorkerID: w.ID,
				Message:  m.msgf("Worker %s may be stuck: no output change for 5+ minutes", "員工 %s 可能卡住了：超過 5 分鐘沒有輸出變化", w.Name),
			})
			// Reset the timer so we don't spam warnings every 60s
			m.lastPaneContent[w.ID] = paneSnapshot{content: content, since: time.Now()}
		}
	}
}
