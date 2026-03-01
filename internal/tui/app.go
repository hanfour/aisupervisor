package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hanfourmini/aisupervisor/internal/session"
	"github.com/hanfourmini/aisupervisor/internal/supervisor"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

type view int

const (
	dashboardView view = iota
	sessionDetailView
	addSessionView
	confirmView
	rolesListView
)

type App struct {
	currentView   view
	dashboard     dashboardModel
	sessionDetail sessionDetailModel
	addSession    addSessionModel
	confirm       confirmModel
	roles         rolesModel

	supervisor    *supervisor.Supervisor
	tmuxClient    tmux.TmuxClient
	sessionMgr    *session.Manager
	sessions      []*session.MonitoredSession
	ctx           context.Context
	cancel        context.CancelFunc
	width, height int
}

func NewApp(
	sup *supervisor.Supervisor,
	client tmux.TmuxClient,
	mgr *session.Manager,
	sessions []*session.MonitoredSession,
) *App {
	ctx, cancel := context.WithCancel(context.Background())

	// Get roles from role manager
	roleList := sup.RoleManager().List()

	return &App{
		currentView: dashboardView,
		dashboard:   newDashboardModel(sessions),
		roles:       newRolesModel(roleList),
		supervisor:  sup,
		tmuxClient:  client,
		sessionMgr:  mgr,
		sessions:    sessions,
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.listenEvents(),
		a.tickPaneContent(),
	)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			a.cancel()
			return a, tea.Quit
		case "esc":
			if a.currentView != dashboardView {
				a.currentView = dashboardView
				return a, nil
			}
		case "enter":
			if a.currentView == dashboardView {
				if s := a.dashboard.selectedSession(); s != nil {
					a.sessionDetail = newSessionDetailModel(s)
					a.currentView = sessionDetailView
					return a, nil
				}
			}
		case "a":
			if a.currentView == dashboardView {
				tmuxSessions, _ := a.tmuxClient.ListSessions()
				a.addSession = newAddSessionModel(tmuxSessions)
				a.currentView = addSessionView
				return a, a.addSession.Init()
			}
		case "r":
			if a.currentView == dashboardView {
				a.roles = newRolesModel(a.supervisor.RoleManager().List())
				a.currentView = rolesListView
				return a, nil
			}
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

	case supervisorEventMsg:
		a.dashboard, _ = a.dashboard.Update(msg)
		a.roles, _ = a.roles.Update(msg)
		if a.currentView == sessionDetailView {
			a.sessionDetail, _ = a.sessionDetail.Update(msg)
		}
		// If paused (low confidence), show confirm dialog
		e := supervisor.Event(msg)
		if e.Type == supervisor.EventPaused {
			a.confirm = newConfirmModel(e)
			a.currentView = confirmView
		}
		return a, a.listenEvents()

	case paneContentMsg:
		if a.currentView == sessionDetailView {
			a.sessionDetail, _ = a.sessionDetail.Update(msg)
		}
		return a, nil
	}

	// Delegate to current view
	var cmd tea.Cmd
	switch a.currentView {
	case dashboardView:
		a.dashboard, cmd = a.dashboard.Update(msg)
	case sessionDetailView:
		a.sessionDetail, cmd = a.sessionDetail.Update(msg)
	case addSessionView:
		a.addSession, cmd = a.addSession.Update(msg)
		if a.addSession.done && a.addSession.result != nil {
			if a.sessionMgr != nil {
				_ = a.sessionMgr.Add(a.addSession.result)
			}
			a.sessions = append(a.sessions, a.addSession.result)
			a.dashboard = newDashboardModel(a.sessions)
			// Start monitoring new session
			go a.supervisor.Monitor(a.ctx, a.addSession.result)
			a.currentView = dashboardView
		}
	case confirmView:
		a.confirm, cmd = a.confirm.Update(msg)
		if a.confirm.decided {
			// Send the chosen response
			if a.confirm.chosen != nil {
				s := a.findSession(a.confirm.event.SessionID)
				if s != nil {
					tmuxSender := tmux.NewSender(a.tmuxClient)
					_ = tmuxSender.SendWithEnter(s.TmuxSession, s.Window, s.Pane, a.confirm.chosen.Key)
				}
			}
			a.currentView = dashboardView
		}
	case rolesListView:
		a.roles, cmd = a.roles.Update(msg)
	}

	return a, cmd
}

func (a *App) View() string {
	switch a.currentView {
	case sessionDetailView:
		return a.sessionDetail.View()
	case addSessionView:
		return a.addSession.View()
	case confirmView:
		return a.confirm.View()
	case rolesListView:
		return a.roles.View()
	default:
		return a.dashboard.View()
	}
}

func (a *App) listenEvents() tea.Cmd {
	return func() tea.Msg {
		select {
		case e, ok := <-a.supervisor.Events():
			if !ok {
				return nil
			}
			return supervisorEventMsg(e)
		case <-a.ctx.Done():
			return nil
		}
	}
}

func (a *App) tickPaneContent() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		if a.currentView == sessionDetailView && a.sessionDetail.session != nil {
			s := a.sessionDetail.session
			content, err := a.tmuxClient.CapturePane(s.TmuxSession, s.Window, s.Pane, 100)
			if err == nil {
				return paneContentMsg{SessionID: s.ID, Content: content}
			}
		}
		return nil
	})
}

func (a *App) findSession(id string) *session.MonitoredSession {
	for _, s := range a.sessions {
		if s.ID == id {
			return s
		}
	}
	return nil
}
