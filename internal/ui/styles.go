package ui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#89b4fa")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cdd6f4")).
			Background(lipgloss.Color("#313244")).
			Padding(0, 1).
			Width(200)

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f38ba8")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a6e3a1")).
			Padding(0, 1)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7f849c"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9399b2")).
			Padding(0, 1)

	unreadStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#f9e2af"))

	selRowStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#11111b")).
			Background(lipgloss.Color("#89b4fa"))

	selectedMarkStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#89b4fa")).
				Bold(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94e2d5")).
			Width(10)
)
