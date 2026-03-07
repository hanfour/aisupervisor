package gui

import (
	"context"
	"fmt"

	"github.com/hanfourmini/aisupervisor/internal/config"
	"github.com/hanfourmini/aisupervisor/internal/group"
	"github.com/hanfourmini/aisupervisor/internal/role"
	"github.com/hanfourmini/aisupervisor/internal/session"
	"github.com/hanfourmini/aisupervisor/internal/supervisor"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

// App is the Wails application binding. Methods are callable from the frontend.
type App struct {
	ctx        context.Context
	sup        *supervisor.Supervisor
	sessionMgr *session.Manager
	tmuxClient tmux.TmuxClient
	cfg        *config.Config
	groupMgr   *group.Manager
	resolver   *role.SessionRoleResolver
	sessions   []*session.MonitoredSession
}

// NewApp creates a new GUI application binding.
func NewApp(
	sup *supervisor.Supervisor,
	sessionMgr *session.Manager,
	tmuxClient tmux.TmuxClient,
	cfg *config.Config,
	groupMgr *group.Manager,
	resolver *role.SessionRoleResolver,
	sessions []*session.MonitoredSession,
) *App {
	return &App{
		sup:        sup,
		sessionMgr: sessionMgr,
		tmuxClient: tmuxClient,
		cfg:        cfg,
		groupMgr:   groupMgr,
		resolver:   resolver,
		sessions:   sessions,
	}
}

// Startup is called by Wails when the application starts.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	go startEventForwarding(ctx, a.sup, a.groupMgr)
}

// Shutdown is called by Wails when the application shuts down.
func (a *App) Shutdown(ctx context.Context) {
	// cleanup if needed
}

// GetSessions returns all monitored sessions.
func (a *App) GetSessions() []SessionDTO {
	dtos := make([]SessionDTO, len(a.sessions))
	for i, s := range a.sessions {
		dtos[i] = SessionToDTO(s)
	}
	return dtos
}

// ClearSessions removes all tracked sessions (used after clearing all projects).
func (a *App) ClearSessions() {
	a.sessions = nil
}

// GetRoles returns all configured roles.
func (a *App) GetRoles() []RoleDTO {
	roles := a.sup.RoleManager().List()
	dtos := make([]RoleDTO, len(roles))
	for i, r := range roles {
		dtos[i] = RoleToDTO(r)
	}
	return dtos
}

// GetSessionRoles returns the role IDs assigned to a session.
func (a *App) GetSessionRoles(sessionID string) []string {
	if a.resolver == nil {
		return nil
	}
	return a.resolver.GetSessionRoleIDs(sessionID)
}

// SetSessionRoles updates the role binding for a session.
func (a *App) SetSessionRoles(sessionID string, roleIDs []string) error {
	if a.resolver == nil {
		return fmt.Errorf("session role resolver not configured")
	}
	a.resolver.SetSessionRoles(sessionID, roleIDs)
	return nil
}

// GetGroups returns all configured groups.
func (a *App) GetGroups() []GroupDTO {
	if a.groupMgr == nil {
		return nil
	}
	groups := a.groupMgr.Groups()
	dtos := make([]GroupDTO, len(groups))
	for i, g := range groups {
		dtos[i] = GroupToDTO(g)
	}
	return dtos
}

// GetActiveDiscussions returns currently active group discussions.
func (a *App) GetActiveDiscussions() []DiscussionDTO {
	if a.groupMgr == nil {
		return nil
	}
	discussions := a.groupMgr.ActiveDiscussions()
	dtos := make([]DiscussionDTO, len(discussions))
	for i, d := range discussions {
		dtos[i] = DiscussionToDTO(d)
	}
	return dtos
}

// ApproveEvent approves a paused event for the given session.
func (a *App) ApproveEvent(sessionID string, optionKey string) error {
	return a.sup.ApprovePaused(sessionID, optionKey)
}

// GetConfig returns the current configuration.
func (a *App) GetConfig() *config.Config {
	return a.cfg
}
