package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hanfourmini/aisupervisor/internal/detector"
	"github.com/hanfourmini/aisupervisor/internal/supervisor"
)

type confirmModel struct {
	event   supervisor.Event
	options []detector.ResponseOption
	cursor  int
	decided bool
	chosen  *detector.ResponseOption
}

func newConfirmModel(e supervisor.Event) confirmModel {
	var options []detector.ResponseOption
	if e.Match != nil {
		options = e.Match.Options
	}
	return confirmModel{
		event:   e,
		options: options,
	}
}

func (m confirmModel) Init() tea.Cmd {
	return nil
}

func (m confirmModel) Update(msg tea.Msg) (confirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter":
			if m.cursor < len(m.options) {
				m.decided = true
				m.chosen = &m.options[m.cursor]
			}
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Manual Decision Required"))
	b.WriteString("\n\n")

	b.WriteString(normalStyle.Render(fmt.Sprintf("Session: %s", m.event.SessionName)))
	b.WriteString("\n")

	if m.event.Match != nil {
		b.WriteString(normalStyle.Render(fmt.Sprintf("Prompt: %s", m.event.Match.Summary)))
		b.WriteString("\n")
	}

	if m.event.Decision != nil {
		b.WriteString(eventPending.Render(fmt.Sprintf("AI suggested: %s (confidence: %.0f%%)", m.event.Decision.ChosenOption.Label, m.event.Decision.Confidence*100)))
		b.WriteString("\n")
		b.WriteString(normalStyle.Render(fmt.Sprintf("Reasoning: %s", m.event.Decision.Reasoning)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Choose response:"))
	b.WriteString("\n")

	for i, opt := range m.options {
		if i == m.cursor {
			b.WriteString(selectedStyle.Render(fmt.Sprintf("> [%s] %s", opt.Key, opt.Label)))
		} else {
			b.WriteString(normalStyle.Render(fmt.Sprintf("  [%s] %s", opt.Key, opt.Label)))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k: navigate | enter: send | esc: skip"))

	return b.String()
}
