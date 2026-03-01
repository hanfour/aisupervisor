package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("229")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	statusActive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")).
			Bold(true)

	statusPaused = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")).
			Bold(true)

	statusStopped = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	eventApproved = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	eventDenied = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	eventPending = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226"))

	eventError = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2)

	paneContentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250")).
				Background(lipgloss.Color("235")).
				Padding(0, 1)
)
