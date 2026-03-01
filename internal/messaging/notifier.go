package messaging

import (
	"context"
	"fmt"

	"github.com/hanfourmini/aisupervisor/internal/company"
)

// Notifier subscribes to company events and forwards them to messengers.
type Notifier struct {
	companyMgr *company.Manager
	messengers []Messenger
	router     *Router
}

func NewNotifier(mgr *company.Manager, messengers []Messenger) *Notifier {
	router := NewRouter(mgr)
	n := &Notifier{
		companyMgr: mgr,
		messengers: messengers,
		router:     router,
	}

	// Register command handler on all messengers
	for _, m := range messengers {
		m.OnCommand(router.Handle)
	}

	return n
}

// Start begins forwarding events and starts all messengers.
func (n *Notifier) Start(ctx context.Context) error {
	// Start all messengers
	for _, m := range n.messengers {
		go m.Start(ctx)
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
				msg := formatEvent(e)
				for _, m := range n.messengers {
					m.SendNotification(msg)
				}
			}
		}
	}()

	return nil
}

func formatEvent(e company.Event) string {
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}
