package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hanfourmini/aisupervisor/internal/session"
	"github.com/hanfourmini/aisupervisor/internal/supervisor"
)

type dashboardModel struct {
	sessions []*session.MonitoredSession
	events   []supervisor.Event
	cursor   int
	width    int
	height   int
}

func newDashboardModel(sessions []*session.MonitoredSession) dashboardModel {
	return dashboardModel{
		sessions: sessions,
		events:   make([]supervisor.Event, 0),
	}
}

func (m dashboardModel) Init() tea.Cmd {
	return nil
}

func (m dashboardModel) Update(msg tea.Msg) (dashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
			}
		}
	case supervisorEventMsg:
		m.events = append(m.events, supervisor.Event(msg))
		if len(m.events) > 50 {
			m.events = m.events[len(m.events)-50:]
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m dashboardModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("aisupervisor Dashboard"))
	b.WriteString("\n\n")

	// Session table
	header := fmt.Sprintf("  %-20s %-25s %-10s %-8s", "NAME", "TMUX TARGET", "TYPE", "STATUS")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for i, s := range m.sessions {
		tmuxRef := fmt.Sprintf("%s:%d.%d", s.TmuxSession, s.Window, s.Pane)
		statusStr := renderStatus(s.Status)
		line := fmt.Sprintf("%-20s %-25s %-10s %s", s.Name, tmuxRef, s.ToolType, statusStr)

		if i == m.cursor {
			b.WriteString(selectedStyle.Render("> " + line))
		} else {
			b.WriteString(normalStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}

	if len(m.sessions) == 0 {
		b.WriteString(normalStyle.Render("  No sessions. Press 'a' to add one."))
		b.WriteString("\n")
	}

	// Recent events
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Recent Events"))
	b.WriteString("\n")

	start := 0
	maxEvents := 10
	if len(m.events) > maxEvents {
		start = len(m.events) - maxEvents
	}
	for _, e := range m.events[start:] {
		b.WriteString(renderEvent(e))
		b.WriteString("\n")
	}

	if len(m.events) == 0 {
		b.WriteString(normalStyle.Render("  Waiting for events..."))
		b.WriteString("\n")
	}

	// Help
	b.WriteString(helpStyle.Render("j/k: navigate | enter: details | a: add session | r: roles | c: company | q: quit"))

	return b.String()
}

func (m dashboardModel) selectedSession() *session.MonitoredSession {
	if m.cursor >= 0 && m.cursor < len(m.sessions) {
		return m.sessions[m.cursor]
	}
	return nil
}

func renderStatus(s session.Status) string {
	switch s {
	case session.StatusActive:
		return statusActive.Render("active")
	case session.StatusPaused:
		return statusPaused.Render("paused")
	case session.StatusStopped:
		return statusStopped.Render("stopped")
	default:
		return string(s)
	}
}

func renderEvent(e supervisor.Event) string {
	ts := e.Timestamp.Format("15:04:05")
	switch e.Type {
	case supervisor.EventDetected:
		return eventPending.Render(fmt.Sprintf("  [%s] DETECTED %s: %s", ts, e.SessionName, e.Match.Summary))
	case supervisor.EventDecision:
		return eventApproved.Render(fmt.Sprintf("  [%s] DECIDED  %s: %s (%.0f%%)", ts, e.SessionName, e.Decision.ChosenOption.Label, e.Decision.Confidence*100))
	case supervisor.EventAutoApproved:
		return eventApproved.Render(fmt.Sprintf("  [%s] AUTO     %s: %s", ts, e.SessionName, e.Decision.Reasoning))
	case supervisor.EventSent:
		return eventApproved.Render(fmt.Sprintf("  [%s] SENT     %s: key=%s", ts, e.SessionName, e.Decision.ChosenOption.Key))
	case supervisor.EventPaused:
		return eventPending.Render(fmt.Sprintf("  [%s] PAUSED   %s: low confidence (%.0f%%)", ts, e.SessionName, e.Decision.Confidence*100))
	case supervisor.EventError:
		return eventError.Render(fmt.Sprintf("  [%s] ERROR    %s: %v", ts, e.SessionName, e.Error))
	case supervisor.EventRoleIntervention:
		roleID := e.RoleID
		reasoning := ""
		if e.Intervention != nil {
			reasoning = e.Intervention.Reasoning
			if len(reasoning) > 50 {
				reasoning = reasoning[:50] + "..."
			}
		}
		return eventApproved.Render(fmt.Sprintf("  [%s] ROLE     %s [%s]: %s", ts, e.SessionName, roleID, reasoning))
	case supervisor.EventRoleObservation:
		return eventPending.Render(fmt.Sprintf("  [%s] OBSERVE  %s", ts, e.SessionName))
	default:
		return fmt.Sprintf("  [%s] %s", ts, e.Type)
	}
}
