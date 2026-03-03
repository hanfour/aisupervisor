package gui

import (
	"context"

	"github.com/hanfourmini/aisupervisor/internal/company"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// startCompanyEventForwarding forwards company events to the Wails frontend.
func startCompanyEventForwarding(ctx context.Context, mgr *company.Manager) {
	ch := mgr.Subscribe()
	defer mgr.Unsubscribe(ch)
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			dto := CompanyEventToDTO(e)
			wailsRuntime.EventsEmit(ctx, "company:event", dto)

			// Forward personality-specific events with dedicated channels
			switch e.Type {
			case company.EventNarrativeGenerated:
				wailsRuntime.EventsEmit(ctx, "personality:narrative", map[string]string{
					"workerId": e.WorkerID,
					"message":  e.Message,
				})
			case company.EventMoodChanged:
				wailsRuntime.EventsEmit(ctx, "personality:mood", map[string]string{
					"workerId": e.WorkerID,
					"message":  e.Message,
				})
			case company.EventRelationshipUpdated:
				wailsRuntime.EventsEmit(ctx, "personality:relationship", map[string]string{
					"workerId": e.WorkerID,
					"message":  e.Message,
				})
			}
		}
	}
}
