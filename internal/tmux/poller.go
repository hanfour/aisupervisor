package tmux

import (
	"context"
	"sync"
	"time"
)

type PaneUpdate struct {
	SessionName string
	WindowIndex int
	PaneIndex   int
	Content     string
	Timestamp   time.Time
}

type Poller struct {
	client   TmuxClient
	interval time.Duration
	lines    int

	mu       sync.RWMutex
	lastContent map[string]string
}

func NewPoller(client TmuxClient, intervalMs, contextLines int) *Poller {
	return &Poller{
		client:      client,
		interval:    time.Duration(intervalMs) * time.Millisecond,
		lines:       contextLines,
		lastContent: make(map[string]string),
	}
}

func (p *Poller) paneKey(session string, window, pane int) string {
	return session + ":" + string(rune('0'+window)) + ":" + string(rune('0'+pane))
}

func (p *Poller) Poll(ctx context.Context, session string, window, pane int, updates chan<- PaneUpdate) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	key := p.paneKey(session, window, pane)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			content, err := p.client.CapturePane(session, window, pane, p.lines)
			if err != nil {
				continue
			}

			p.mu.RLock()
			last := p.lastContent[key]
			p.mu.RUnlock()

			if content == last {
				continue
			}

			p.mu.Lock()
			p.lastContent[key] = content
			p.mu.Unlock()

			updates <- PaneUpdate{
				SessionName: session,
				WindowIndex: window,
				PaneIndex:   pane,
				Content:     content,
				Timestamp:   time.Now(),
			}
		}
	}
}
