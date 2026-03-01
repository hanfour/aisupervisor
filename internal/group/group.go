package group

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"

	"github.com/hanfourmini/aisupervisor/internal/audit"
	"github.com/hanfourmini/aisupervisor/internal/role"
)

// Group defines a set of roles that collaborate on decisions.
type Group struct {
	ID                  string   `yaml:"id" json:"id"`
	Name                string   `yaml:"name" json:"name"`
	LeaderID            string   `yaml:"leader_id" json:"leaderId"`
	RoleIDs             []string `yaml:"role_ids" json:"roleIds"`
	DivergenceThreshold float64  `yaml:"divergence_threshold" json:"divergenceThreshold"`
}

// SessionRoleFilter is a function that filters roles for a specific session.
// Returns nil if no filtering should be applied (use all roles).
type SessionRoleFilter func(sessionID string) []role.Role

// ManagerOption configures the group Manager.
type ManagerOption func(*Manager)

// WithSessionFilter sets a session role filter for group evaluation.
func WithSessionFilter(f SessionRoleFilter) ManagerOption {
	return func(m *Manager) { m.sessionFilter = f }
}

// WithAuditor sets an audit logger for discussion events.
func WithAuditor(a *audit.Logger) ManagerOption {
	return func(m *Manager) { m.auditor = a }
}

// Manager coordinates group-based evaluation with two-phase discussions.
type Manager struct {
	mu                sync.RWMutex
	groups            map[string]*Group
	roleManager       *role.Manager
	sessionFilter     SessionRoleFilter
	auditor           *audit.Logger
	discussionEvents  chan DiscussionEvent
	activeDiscussions map[string]*Discussion
}

// NewManager creates a group manager.
// An optional SessionRoleFilter can be provided to filter group roles per session.
func NewManager(rm *role.Manager, groups []*Group, opts ...ManagerOption) *Manager {
	gm := &Manager{
		groups:            make(map[string]*Group),
		roleManager:       rm,
		discussionEvents:  make(chan DiscussionEvent, 200),
		activeDiscussions: make(map[string]*Discussion),
	}
	for _, opt := range opts {
		opt(gm)
	}
	for _, g := range groups {
		gm.groups[g.ID] = g
	}
	return gm
}

// DiscussionEvents returns a channel for receiving discussion events.
func (m *Manager) DiscussionEvents() <-chan DiscussionEvent {
	return m.discussionEvents
}

// Groups returns all configured groups.
func (m *Manager) Groups() []*Group {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Group, 0, len(m.groups))
	for _, g := range m.groups {
		result = append(result, g)
	}
	return result
}

// ActiveDiscussions returns all currently active discussions.
func (m *Manager) ActiveDiscussions() []*Discussion {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Discussion, 0, len(m.activeDiscussions))
	for _, d := range m.activeDiscussions {
		result = append(result, d)
	}
	return result
}

// EvaluateWithGroups runs evaluation through all groups.
// If no groups match, falls back to standard role evaluation.
func (m *Manager) EvaluateWithGroups(ctx context.Context, obs role.Observation, sessionID string) (*role.Intervention, error) {
	m.mu.RLock()
	groups := make([]*Group, 0, len(m.groups))
	for _, g := range m.groups {
		groups = append(groups, g)
	}
	m.mu.RUnlock()

	if len(groups) == 0 {
		return m.roleManager.EvaluateReactive(ctx, obs)
	}

	// Try each group; use the first one that has relevant roles
	for _, grp := range groups {
		roles := m.resolveGroupRolesForSession(grp, sessionID)
		if len(roles) == 0 {
			continue
		}

		// Check if any role in this group would evaluate
		hasCandidate := false
		for _, r := range roles {
			mode := r.Mode()
			if (mode == role.ModeReactive || mode == role.ModeHybrid) && r.ShouldEvaluate(obs) {
				hasCandidate = true
				break
			}
		}
		if !hasCandidate {
			continue
		}

		iv, err := m.RunDiscussion(ctx, grp, obs, sessionID)
		if err != nil {
			return nil, fmt.Errorf("group %s discussion: %w", grp.ID, err)
		}
		if iv != nil {
			return iv, nil
		}
	}

	// No group matched, fallback
	return m.roleManager.EvaluateReactive(ctx, obs)
}

// resolveGroupRoles returns the Role instances for a group, sorted by priority descending.
func (m *Manager) resolveGroupRoles(grp *Group) []role.Role {
	return m.resolveGroupRolesForSession(grp, "")
}

// resolveGroupRolesForSession returns roles for a group, optionally filtered by session.
func (m *Manager) resolveGroupRolesForSession(grp *Group, sessionID string) []role.Role {
	// Get session-allowed roles if filter exists
	var allowedIDs map[string]bool
	if sessionID != "" && m.sessionFilter != nil {
		sessionRoles := m.sessionFilter(sessionID)
		if sessionRoles != nil {
			allowedIDs = make(map[string]bool, len(sessionRoles))
			for _, r := range sessionRoles {
				allowedIDs[r.ID()] = true
			}
		}
	}

	var roles []role.Role
	for _, id := range grp.RoleIDs {
		// Skip if session filter excludes this role
		if allowedIDs != nil && !allowedIDs[id] {
			continue
		}
		if r, ok := m.roleManager.Get(id); ok {
			roles = append(roles, r)
		}
	}
	sort.Slice(roles, func(i, j int) bool {
		return roles[i].Priority() > roles[j].Priority()
	})
	return roles
}

func (m *Manager) emitEvent(e DiscussionEvent) {
	select {
	case m.discussionEvents <- e:
	default:
	}

	// Persist to audit log
	if m.auditor != nil {
		if err := m.auditor.LogDiscussion(audit.DiscussionEntry{
			Timestamp:    e.Timestamp,
			DiscussionID: e.DiscussionID,
			SessionID:    e.SessionID,
			GroupID:      e.GroupID,
			Phase:        string(e.Phase),
			RoleID:       e.RoleID,
			RoleName:     e.RoleName,
			Action:       e.Action,
			Message:      e.Message,
			Confidence:   e.Confidence,
		}); err != nil {
			log.Printf("audit discussion error: %v", err)
		}
	}
}
