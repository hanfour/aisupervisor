package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/hanfourmini/aisupervisor/internal/session"
	"github.com/hanfourmini/aisupervisor/internal/tmux"
)

type addSessionModel struct {
	inputs       []textinput.Model
	focusIndex   int
	tmuxSessions []tmux.SessionInfo
	done         bool
	result       *session.MonitoredSession
	width        int
}

const (
	fieldName = iota
	fieldTmuxSession
	fieldGoal
	fieldProjectDir
)

func newAddSessionModel(tmuxSessions []tmux.SessionInfo) addSessionModel {
	inputs := make([]textinput.Model, 4)

	inputs[fieldName] = textinput.New()
	inputs[fieldName].Placeholder = "session display name"
	inputs[fieldName].Focus()
	inputs[fieldName].CharLimit = 30

	inputs[fieldTmuxSession] = textinput.New()
	inputs[fieldTmuxSession].Placeholder = "tmux session name (e.g. main:0.0)"
	inputs[fieldTmuxSession].CharLimit = 50

	inputs[fieldGoal] = textinput.New()
	inputs[fieldGoal].Placeholder = "task goal (optional)"
	inputs[fieldGoal].CharLimit = 200

	inputs[fieldProjectDir] = textinput.New()
	inputs[fieldProjectDir].Placeholder = "project directory (optional, e.g. ~/projects/myapp)"
	inputs[fieldProjectDir].CharLimit = 200

	return addSessionModel{
		inputs:       inputs,
		tmuxSessions: tmuxSessions,
	}
}

func (m addSessionModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m addSessionModel) Update(msg tea.Msg) (addSessionModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab", "down", "up":
			if msg.String() == "up" || msg.String() == "shift+tab" {
				m.focusIndex--
				if m.focusIndex < 0 {
					m.focusIndex = len(m.inputs) - 1
				}
			} else {
				m.focusIndex++
				if m.focusIndex >= len(m.inputs) {
					m.focusIndex = 0
				}
			}
			cmds := make([]tea.Cmd, len(m.inputs))
			for i := range m.inputs {
				if i == m.focusIndex {
					cmds[i] = m.inputs[i].Focus()
				} else {
					m.inputs[i].Blur()
				}
			}
			return m, tea.Batch(cmds...)
		case "enter":
			if m.inputs[fieldName].Value() != "" && m.inputs[fieldTmuxSession].Value() != "" {
				m.done = true
				m.result = &session.MonitoredSession{
					Name:       m.inputs[fieldName].Value(),
					ToolType:   "auto",
					TaskGoal:   m.inputs[fieldGoal].Value(),
					ProjectDir: m.inputs[fieldProjectDir].Value(),
					Status:     session.StatusActive,
				}
				// Parse tmux ref
				tmuxRef := m.inputs[fieldTmuxSession].Value()
				var sessName string
				var window, pane int
				n, _ := fmt.Sscanf(tmuxRef, "%[^:]:%d.%d", &sessName, &window, &pane)
				if n == 0 {
					sessName = tmuxRef
				}
				m.result.TmuxSession = sessName
				m.result.Window = window
				m.result.Pane = pane
			}
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	}

	// Update focused input
	var cmd tea.Cmd
	m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	return m, cmd
}

func (m addSessionModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Add Session"))
	b.WriteString("\n\n")

	labels := []string{"Name:", "Tmux Target:", "Task Goal:", "Project Dir:"}
	for i, input := range m.inputs {
		b.WriteString(normalStyle.Render(labels[i]))
		b.WriteString("\n")
		b.WriteString(input.View())
		b.WriteString("\n\n")
	}

	// Show available tmux sessions
	if len(m.tmuxSessions) > 0 {
		b.WriteString(headerStyle.Render("Available tmux sessions:"))
		b.WriteString("\n")
		for _, ts := range m.tmuxSessions {
			b.WriteString(normalStyle.Render(fmt.Sprintf("  %s (%d windows)", ts.Name, ts.Windows)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("tab: next field | enter: create | esc: cancel"))

	return b.String()
}
