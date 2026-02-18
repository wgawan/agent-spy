package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("62")).
		Padding(0, 1)

	statsStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Background(lipgloss.Color("236")).
		Padding(0, 1)

	addedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")) // green

	deletedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")) // red

	selectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("62"))

	normalStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	borderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	headerStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("62"))

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	diffAddStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("10"))

	diffDelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("9"))

	diffContextStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	diffHunkStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("14"))
)
