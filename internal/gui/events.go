package gui

import (
	"context"

	"github.com/hanfourmini/aisupervisor/internal/group"
	"github.com/hanfourmini/aisupervisor/internal/supervisor"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// startEventForwarding forwards supervisor and discussion events to the Wails frontend.
func startEventForwarding(ctx context.Context, sup *supervisor.Supervisor, groupMgr *group.Manager) {
	// Forward supervisor events
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-sup.Events():
				if !ok {
					return
				}
				dto := EventToDTO(e)
				wailsRuntime.EventsEmit(ctx, "supervisor:event", dto)
				if e.Type == supervisor.EventError && e.Error != nil {
					wailsRuntime.EventsEmit(ctx, "supervisor:error", e.Error.Error())
				}
			}
		}
	}()

	// Forward discussion events
	if groupMgr != nil {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case e, ok := <-groupMgr.DiscussionEvents():
					if !ok {
						return
					}
					dto := DiscussionEventToDTO(e)
					wailsRuntime.EventsEmit(ctx, "discussion:event", dto)
				}
			}
		}()
	}
}
