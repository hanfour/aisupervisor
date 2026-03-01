package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hanfourmini/aisupervisor/internal/session"
	"github.com/hanfourmini/aisupervisor/internal/supervisor"
)

type sessionDetailModel struct {
	session     *session.MonitoredSession
	paneContent string
	events      []supervisor.Event
	scrollPos   int
	width       int
	height      int
}

func newSessionDetailModel(s *session.MonitoredSession) sessionDetailModel {
	return sessionDetailModel{
		session: s,
		events:  make([]supervisor.Event, 0),
	}
}

func (m sessionDetailModel) Init() tea.Cmd {
	return nil
}

func (m sessionDetailModel) Update(msg tea.Msg) (sessionDetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.scrollPos > 0 {
				m.scrollPos--
			}
		case "down", "j":
			m.scrollPos++
		}
	case paneContentMsg:
		if msg.SessionID == m.session.ID {
			m.paneContent = msg.Content
		}
	case supervisorEventMsg:
		e := supervisor.Event(msg)
		if e.SessionID == m.session.ID {
			m.events = append(m.events, e)
			if len(m.events) > 30 {
				m.events = m.events[len(m.events)-30:]
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m sessionDetailModel) View() string {
	var b strings.Builder

	tmuxRef := fmt.Sprintf("%s:%d.%d", m.session.TmuxSession, m.session.Window, m.session.Pane)
	title := fmt.Sprintf("Session: %s (%s)", m.session.Name, tmuxRef)
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Session info
	info := fmt.Sprintf("Type: %s | Status: %s", m.session.ToolType, renderStatus(m.session.Status))
	if m.session.TaskGoal != "" {
		info += fmt.Sprintf(" | Goal: %s", m.session.TaskGoal)
	}
	b.WriteString(normalStyle.Render(info))
	b.WriteString("\n\n")

	// Pane content
	b.WriteString(headerStyle.Render("Pane Content (live)"))
	b.WriteString("\n")

	if m.paneContent != "" {
		lines := strings.Split(m.paneContent, "\n")
		maxLines := 15
		if m.height > 30 {
			maxLines = m.height - 20
		}

		start := m.scrollPos
		if start >= len(lines) {
			start = len(lines) - 1
		}
		if start < 0 {
			start = 0
		}
		end := start + maxLines
		if end > len(lines) {
			end = len(lines)
		}

		content := strings.Join(lines[start:end], "\n")
		b.WriteString(paneContentStyle.Render(content))
	} else {
		b.WriteString(normalStyle.Render("  Waiting for pane content..."))
	}
	b.WriteString("\n\n")

	// Events for this session
	b.WriteString(headerStyle.Render("Session Events"))
	b.WriteString("\n")

	start := 0
	if len(m.events) > 5 {
		start = len(m.events) - 5
	}
	for _, e := range m.events[start:] {
		b.WriteString(renderEvent(e))
		b.WriteString("\n")
	}

	// Role interventions for this session
	var roleEvents []supervisor.Event
	for _, e := range m.events {
		if e.Type == supervisor.EventRoleIntervention {
			roleEvents = append(roleEvents, e)
		}
	}
	if len(roleEvents) > 0 {
		b.WriteString("\n")
		b.WriteString(headerStyle.Render("Role Interventions"))
		b.WriteString("\n")
		rStart := 0
		if len(roleEvents) > 5 {
			rStart = len(roleEvents) - 5
		}
		for _, e := range roleEvents[rStart:] {
			roleID := e.RoleID
			reasoning := ""
			if e.Intervention != nil {
				reasoning = e.Intervention.Reasoning
				if len(reasoning) > 60 {
					reasoning = reasoning[:60] + "..."
				}
			}
			line := fmt.Sprintf("  [%s] %s: %s", e.Timestamp.Format("15:04:05"), roleID, reasoning)
			b.WriteString(eventApproved.Render(line))
			b.WriteString("\n")
		}
	}

	b.WriteString(helpStyle.Render("j/k: scroll | esc: back to dashboard | q: quit"))

	return b.String()
}
