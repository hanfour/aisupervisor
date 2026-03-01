package role

import (
	"sync"

	"github.com/hanfourmini/aisupervisor/internal/config"
)

// SessionRoleResolver resolves which roles apply to a specific session.
// It wraps a global Manager and applies session-level filtering without
// copying role instances.
type SessionRoleResolver struct {
	mu              sync.RWMutex
	globalManager   *Manager
	sessionBindings map[string][]string // sessionID → role IDs
}

// NewResolver creates a resolver from a global manager and config bindings.
func NewResolver(mgr *Manager, bindings []config.SessionRoleBinding) *SessionRoleResolver {
	sb := make(map[string][]string)
	for _, b := range bindings {
		sb[b.SessionID] = b.RoleIDs
	}
	return &SessionRoleResolver{
		globalManager:   mgr,
		sessionBindings: sb,
	}
}

// RolesForSession returns the roles applicable to a given session.
// If no binding exists for the session, all global roles are returned.
func (r *SessionRoleResolver) RolesForSession(sessionID string) []Role {
	r.mu.RLock()
	roleIDs, hasBind := r.sessionBindings[sessionID]
	r.mu.RUnlock()

	if !hasBind {
		return r.globalManager.List()
	}

	idSet := make(map[string]bool, len(roleIDs))
	for _, id := range roleIDs {
		idSet[id] = true
	}

	allRoles := r.globalManager.List()
	filtered := make([]Role, 0, len(roleIDs))
	for _, role := range allRoles {
		if idSet[role.ID()] {
			filtered = append(filtered, role)
		}
	}
	return filtered
}

// SetSessionRoles updates the role binding for a session at runtime.
func (r *SessionRoleResolver) SetSessionRoles(sessionID string, roleIDs []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(roleIDs) == 0 {
		delete(r.sessionBindings, sessionID)
	} else {
		r.sessionBindings[sessionID] = roleIDs
	}
}

// GetSessionRoleIDs returns the configured role IDs for a session.
// Returns nil if no binding exists (meaning all roles apply).
func (r *SessionRoleResolver) GetSessionRoleIDs(sessionID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessionBindings[sessionID]
}
