package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hanfourmini/aisupervisor/internal/company"
)

type companyDashModel struct {
	companyMgr *company.Manager
	events     []company.Event
	width      int
	height     int
}

func newCompanyDashModel(mgr *company.Manager) companyDashModel {
	return companyDashModel{
		companyMgr: mgr,
		events:     make([]company.Event, 0),
	}
}

func (m companyDashModel) Init() tea.Cmd {
	return nil
}

func (m companyDashModel) Update(msg tea.Msg) (companyDashModel, tea.Cmd) {
	switch msg := msg.(type) {
	case companyEventMsg:
		m.events = append(m.events, company.Event(msg))
		if len(m.events) > 50 {
			m.events = m.events[len(m.events)-50:]
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m companyDashModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Company Dashboard"))
	b.WriteString("\n\n")

	// Projects table
	b.WriteString(headerStyle.Render("Projects"))
	b.WriteString("\n")

	if m.companyMgr != nil {
		projects := m.companyMgr.ListProjects()
		if len(projects) == 0 {
			b.WriteString(normalStyle.Render("  No projects"))
			b.WriteString("\n")
		} else {
			ph := fmt.Sprintf("  %-20s %-10s %-8s", "NAME", "STATUS", "TASKS")
			b.WriteString(normalStyle.Render(ph))
			b.WriteString("\n")
			for _, p := range projects {
				prog := m.companyMgr.ProjectProgress(p.ID)
				line := fmt.Sprintf("  %-20s %-10s %d/%d", truncate(p.Name, 20), p.Status, prog.Done, prog.Total)
				b.WriteString(normalStyle.Render(line))
				b.WriteString("\n")
			}
		}
	}

	// Workers status
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Workers"))
	b.WriteString("\n")

	if m.companyMgr != nil {
		workers := m.companyMgr.ListWorkers()
		if len(workers) == 0 {
			b.WriteString(normalStyle.Render("  No workers"))
			b.WriteString("\n")
		} else {
			wh := fmt.Sprintf("  %-15s %-12s %-10s %-8s %-20s", "NAME", "TIER", "STATUS", "CLI", "TASK")
			b.WriteString(normalStyle.Render(wh))
			b.WriteString("\n")
			for _, w := range workers {
				taskInfo := "-"
				if w.CurrentTaskID != "" {
					taskInfo = w.CurrentTaskID
				}
				cliTool := w.CLITool
				if cliTool == "" {
					cliTool = "claude"
				}
				statusStr := renderWorkerStatus(string(w.Status))
				tierStr := renderTier(string(w.EffectiveTier()))
				line := fmt.Sprintf("  %-15s %s %s  %-8s %-20s",
					truncate(w.Name, 15), tierStr, statusStr, cliTool, truncate(taskInfo, 20))
				b.WriteString(normalStyle.Render(line))
				b.WriteString("\n")
			}
		}
	}

	// Recent company events
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Recent Events"))
	b.WriteString("\n")

	start := 0
	maxEvents := 10
	if len(m.events) > maxEvents {
		start = len(m.events) - maxEvents
	}
	for _, e := range m.events[start:] {
		b.WriteString(renderCompanyEvent(e))
		b.WriteString("\n")
	}
	if len(m.events) == 0 {
		b.WriteString(normalStyle.Render("  Waiting for events..."))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("esc: back to dashboard | q: quit"))

	return b.String()
}

func renderTier(tier string) string {
	switch tier {
	case "consultant":
		return statusStopped.Render("consultant ")
	case "manager":
		return statusPaused.Render("manager    ")
	case "engineer":
		return statusActive.Render("engineer   ")
	default:
		return normalStyle.Render(fmt.Sprintf("%-11s", tier))
	}
}

func renderWorkerStatus(s string) string {
	switch s {
	case "idle":
		return statusActive.Render("idle   ")
	case "working":
		return statusPaused.Render("working")
	case "waiting":
		return statusPaused.Render("waiting")
	case "error":
		return statusStopped.Render("error  ")
	default:
		return normalStyle.Render(fmt.Sprintf("%-7s", s))
	}
}

func renderCompanyEvent(e company.Event) string {
	ts := e.Timestamp.Format("15:04:05")
	label := strings.ToUpper(strings.ReplaceAll(string(e.Type), "_", " "))
	if len(label) > 12 {
		label = label[:12]
	}
	msg := e.Message
	if len(msg) > 60 {
		msg = msg[:60] + "..."
	}
	return eventApproved.Render(fmt.Sprintf("  [%s] %-12s %s", ts, label, msg))
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}
