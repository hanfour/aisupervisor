package tui

import (
	"github.com/hanfourmini/aisupervisor/internal/supervisor"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

// Tea messages

type supervisorEventMsg supervisor.Event

type paneContentMsg struct {
	SessionID string
	Content   string
}

type tmuxSessionsMsg []tmux.SessionInfo

type errMsg struct {
	err error
}

func (e errMsg) Error() string { return e.err.Error() }

type switchViewMsg string

const (
	viewDashboard     switchViewMsg = "dashboard"
	viewSessionDetail switchViewMsg = "session_detail"
	viewAddSession    switchViewMsg = "add_session"
)
