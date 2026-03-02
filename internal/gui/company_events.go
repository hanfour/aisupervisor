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
		}
	}
}
