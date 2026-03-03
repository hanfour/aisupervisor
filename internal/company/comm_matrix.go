package company

import (
	"github.com/hanfourmini/aisupervisor/internal/worker"
)

// CommunicationMatrix enforces communication rules between workers based on hierarchy.
type CommunicationMatrix struct {
	mgr *Manager
}

// NewCommunicationMatrix creates a new CommunicationMatrix.
func NewCommunicationMatrix(mgr *Manager) *CommunicationMatrix {
	return &CommunicationMatrix{mgr: mgr}
}

// CanCommunicate checks if sender can directly communicate with recipient.
// Rules:
//   - Same tier, same manager → direct
//   - Direct parent-child → direct
//   - Consultant ↔ Manager → direct
//   - Cross-tier without direct relation → must route through sender's manager
func (cm *CommunicationMatrix) CanCommunicate(senderID, recipientID string) bool {
	cm.mgr.mu.RLock()
	defer cm.mgr.mu.RUnlock()

	sender, sOk := cm.mgr.workers[senderID]
	recipient, rOk := cm.mgr.workers[recipientID]
	if !sOk || !rOk {
		return false
	}

	// Direct parent-child relationship
	if sender.ParentID == recipientID || recipient.ParentID == senderID {
		return true
	}

	// Same tier, same manager
	if sender.EffectiveTier() == recipient.EffectiveTier() && sender.ParentID == recipient.ParentID {
		return true
	}

	// Consultant ↔ Manager
	sTier := sender.EffectiveTier()
	rTier := recipient.EffectiveTier()
	if (sTier == worker.TierConsultant && rTier == worker.TierManager) ||
		(sTier == worker.TierManager && rTier == worker.TierConsultant) {
		return true
	}

	return false
}

// RouteMessage returns the ordered list of worker IDs a message must pass through
// from sender to recipient. If direct communication is allowed, returns just [recipientID].
// If routing is needed, returns [sender's manager, ..., recipientID].
func (cm *CommunicationMatrix) RouteMessage(senderID, recipientID string) []string {
	if cm.CanCommunicate(senderID, recipientID) {
		return []string{recipientID}
	}

	cm.mgr.mu.RLock()
	sender, ok := cm.mgr.workers[senderID]
	cm.mgr.mu.RUnlock()

	if !ok || sender.ParentID == "" {
		return []string{recipientID}
	}

	// Route through sender's manager
	route := []string{sender.ParentID}

	// Check if manager can reach recipient directly
	if cm.CanCommunicate(sender.ParentID, recipientID) {
		route = append(route, recipientID)
		return route
	}

	// Fallback: direct delivery (best effort)
	route = append(route, recipientID)
	return route
}
