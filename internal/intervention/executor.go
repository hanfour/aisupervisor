package intervention

import (
	"fmt"
	"log"

	"github.com/hanfourmini/aisupervisor/internal/role"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

// Executor sends interventions to tmux panes.
type Executor struct {
	sender *tmux.Sender
	dryRun bool
}

// NewExecutor creates an intervention executor.
func NewExecutor(sender *tmux.Sender, dryRun bool) *Executor {
	return &Executor{sender: sender, dryRun: dryRun}
}

// Execute sends an intervention to the specified tmux pane.
func (e *Executor) Execute(session string, window, pane int, intervention *role.Intervention) error {
	if e.dryRun {
		log.Printf("[dry-run] Would execute %s intervention from role %s: key=%q text=%q",
			intervention.Type, intervention.RoleID, intervention.OptionKey, intervention.Text)
		return nil
	}

	switch intervention.Type {
	case role.InterventionSelectOption:
		return e.sender.SendWithEnter(session, window, pane, intervention.OptionKey)
	case role.InterventionFreeText:
		if err := e.sender.SendLiteral(session, window, pane, intervention.Text); err != nil {
			return fmt.Errorf("send literal: %w", err)
		}
		return e.sender.Send(session, window, pane, "Enter")
	case role.InterventionNone:
		return nil
	default:
		return fmt.Errorf("unknown intervention type: %s", intervention.Type)
	}
}
