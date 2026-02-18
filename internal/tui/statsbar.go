package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderStatsBar() string {
	elapsed := time.Since(m.startTime)
	elapsedStr := formatDuration(elapsed)

	fileCount := fmt.Sprintf("%d files", len(m.uniqueFiles))
	changes := fmt.Sprintf("+%d -%d", m.totalAdded, m.totalDeleted)
	timer := fmt.Sprintf("▶ %s", elapsedStr)

	parts := []string{fileCount, changes, timer}
	if m.gitAvailable && m.gitBranch != "" {
		parts = append(parts, fmt.Sprintf("git:%s", m.gitBranch))
	}

	title := titleStyle.Render(fmt.Sprintf(" agent-spy: %s ", m.watchPath))

	statsContent := ""
	for i, p := range parts {
		if i > 0 {
			statsContent += " │ "
		}
		statsContent += p
	}
	stats := statsStyle.Width(m.width - lipgloss.Width(title)).Render(statsContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, title, stats)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m > 0 {
		return fmt.Sprintf("%dm %02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
