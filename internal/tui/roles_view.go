package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hanfourmini/aisupervisor/internal/role"
	"github.com/hanfourmini/aisupervisor/internal/supervisor"
)

type rolesModel struct {
	roles        []role.Role
	interventions []supervisor.Event // recent role interventions
	cursor       int
	width        int
	height       int
}

func newRolesModel(roles []role.Role) rolesModel {
	return rolesModel{
		roles:         roles,
		interventions: make([]supervisor.Event, 0),
	}
}

func (m rolesModel) Init() tea.Cmd {
	return nil
}

func (m rolesModel) Update(msg tea.Msg) (rolesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.roles)-1 {
				m.cursor++
			}
		}
	case supervisorEventMsg:
		e := supervisor.Event(msg)
		if e.Type == supervisor.EventRoleIntervention || e.Type == supervisor.EventRoleObservation {
			m.interventions = append(m.interventions, e)
			if len(m.interventions) > 50 {
				m.interventions = m.interventions[len(m.interventions)-50:]
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m rolesModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Roles"))
	b.WriteString("\n\n")

	// Roles table
	header := fmt.Sprintf("  %-25s %-12s %-8s %-10s", "ID", "MODE", "PRI", "STATUS")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for i, r := range m.roles {
		status := statusActive.Render("enabled")
		line := fmt.Sprintf("%-25s %-12s %-8d %s", r.ID(), string(r.Mode()), r.Priority(), status)

		if i == m.cursor {
			b.WriteString(selectedStyle.Render("> " + line))
		} else {
			b.WriteString(normalStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}

	if len(m.roles) == 0 {
		b.WriteString(normalStyle.Render("  No roles configured."))
		b.WriteString("\n")
	}

	// Recent interventions
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Recent Interventions"))
	b.WriteString("\n")

	start := 0
	max := 5
	if len(m.interventions) > max {
		start = len(m.interventions) - max
	}
	for _, e := range m.interventions[start:] {
		ts := e.Timestamp.Format("15:04:05")
		roleID := e.RoleID
		reasoning := ""
		if e.Intervention != nil {
			reasoning = e.Intervention.Reasoning
			if len(reasoning) > 60 {
				reasoning = reasoning[:60] + "..."
			}
		}
		line := fmt.Sprintf("  [%s] %s %s: %s", ts, roleID, e.SessionName, reasoning)
		b.WriteString(eventApproved.Render(line))
		b.WriteString("\n")
	}

	if len(m.interventions) == 0 {
		b.WriteString(normalStyle.Render("  No interventions yet."))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("j/k: navigate | esc: back | q: quit"))

	return b.String()
}
