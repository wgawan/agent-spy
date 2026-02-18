package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderLayout() string {
	statsBar := m.renderStatsBar()
	statsHeight := lipgloss.Height(statsBar)

	helpBar := m.renderHelp()
	helpHeight := lipgloss.Height(helpBar)

	contentHeight := m.height - statsHeight - helpHeight

	if m.fullscreen {
		// Detail pane takes over everything below stats bar
		detail := m.renderDetail(m.width, contentHeight)
		return strings.Join([]string{statsBar, detail, helpBar}, "\n")
	}

	// Split: 35% events, 65% detail
	eventWidth := m.width * 35 / 100
	detailWidth := m.width - eventWidth

	eventList := m.renderEventList(eventWidth, contentHeight)
	detail := m.renderDetail(detailWidth, contentHeight)

	content := lipgloss.JoinHorizontal(lipgloss.Top, eventList, detail)

	return strings.Join([]string{statsBar, content, helpBar}, "\n")
}

func (m Model) renderHelp() string {
	if m.filterMode {
		return helpStyle.Width(m.width).Render(
			" filter: " + m.filterText + "█  [enter: apply] [esc: cancel]",
		)
	}
	return helpStyle.Width(m.width).Render(
		" ↑↓:select  F:fullscreen  f:filter  c:clear  ctrl+d/u:scroll detail  q:quit",
	)
}
