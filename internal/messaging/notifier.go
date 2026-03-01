package messaging

import (
	"context"
	"fmt"

	"github.com/hanfourmini/aisupervisor/internal/company"
)

// EventFilter determines whether an event type should be forwarded.
// An empty filter means all events pass.
type EventFilter map[string]bool

// Passes returns true if the event type is allowed by this filter.
func (f EventFilter) Passes(eventType string) bool {
	if len(f) == 0 {
		return true
	}
	return f[eventType]
}

// NewEventFilter creates a filter from a list of allowed event type strings.
func NewEventFilter(types []string) EventFilter {
	if len(types) == 0 {
		return nil
	}
	f := make(EventFilter, len(types))
	for _, t := range types {
		f[t] = true
	}
	return f
}

// messengerEntry pairs a messenger with its specific event filter.
type messengerEntry struct {
	messenger Messenger
	filter    EventFilter
}

// Notifier subscribes to company events and forwards them to messengers.
type Notifier struct {
	companyMgr   *company.Manager
	entries      []messengerEntry
	globalFilter EventFilter
	router       *Router
}

// NotifierOption configures a Notifier.
type NotifierOption func(*Notifier)

// WithGlobalFilter sets a global event type filter for all messengers.
func WithGlobalFilter(types []string) NotifierOption {
	return func(n *Notifier) {
		n.globalFilter = NewEventFilter(types)
	}
}

func NewNotifier(mgr *company.Manager, messengers []Messenger, opts ...NotifierOption) *Notifier {
	router := NewRouter(mgr)
	entries := make([]messengerEntry, len(messengers))
	for i, m := range messengers {
		entries[i] = messengerEntry{messenger: m}
	}

	n := &Notifier{
		companyMgr: mgr,
		entries:    entries,
		router:     router,
	}

	for _, opt := range opts {
		opt(n)
	}

	// Register command handler on all messengers
	for _, e := range n.entries {
		e.messenger.OnCommand(router.Handle)
	}

	return n
}

// SetMessengerFilter sets a per-messenger event type filter.
func (n *Notifier) SetMessengerFilter(idx int, types []string) {
	if idx >= 0 && idx < len(n.entries) {
		n.entries[idx].filter = NewEventFilter(types)
	}
}

// Start begins forwarding events and starts all messengers.
func (n *Notifier) Start(ctx context.Context) error {
	// Start all messengers
	for _, e := range n.entries {
		go e.messenger.Start(ctx)
	}

	// Subscribe to company events and forward
	ch := n.companyMgr.Subscribe()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-ch:
				if !ok {
					return
				}
				eventType := string(e.Type)
				msg := formatEvent(e)
				for _, entry := range n.entries {
					if !n.shouldNotify(entry, eventType) {
						continue
					}
					entry.messenger.SendNotification(msg)
				}
			}
		}
	}()

	return nil
}

// shouldNotify checks global and per-messenger filters.
func (n *Notifier) shouldNotify(entry messengerEntry, eventType string) bool {
	// Per-messenger filter takes precedence if set
	if len(entry.filter) > 0 {
		return entry.filter.Passes(eventType)
	}
	// Fall back to global filter
	return n.globalFilter.Passes(eventType)
}

func formatEvent(e company.Event) string {
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}
