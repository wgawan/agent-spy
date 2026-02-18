package tui

import (
	"fmt"
	"strings"

	"github.com/wally/agent-spy/internal/types"
)

func (m Model) renderDetail(width, height int) string {
	filtered := m.filteredEvents()
	if len(filtered) == 0 || m.selected >= len(filtered) {
		content := normalStyle.Render("  Select an event to view details")
		return borderStyle.Width(width - 2).Height(height - 2).Render(content)
	}

	var lines []string

	// Show selected file info
	ev := filtered[m.selected]
	header := headerStyle.Render(fmt.Sprintf(" %s %s %s", ev.Op.Symbol(), ev.Path, ev.Timestamp.Format("15:04:05")))
	lines = append(lines, header)

	if !m.currentDiff.Available {
		msg := "  No diff available"
		if m.currentDiff.Error != "" {
			msg = fmt.Sprintf("  %s", m.currentDiff.Error)
		}
		lines = append(lines, normalStyle.Render(msg))
	} else {
		for _, hunk := range m.currentDiff.Hunks {
			lines = append(lines, diffHunkStyle.Render(hunk.Header))
			for _, line := range hunk.Lines {
				rendered := renderDiffLine(line, width-4)
				lines = append(lines, rendered)
			}
		}

		// Stats summary
		stats := fmt.Sprintf("  %s %s",
			addedStyle.Render(fmt.Sprintf("+%d", m.currentDiff.Stats.Added)),
			deletedStyle.Render(fmt.Sprintf("-%d", m.currentDiff.Stats.Deleted)),
		)
		lines = append(lines, "", stats)
	}

	// Apply scroll offset
	if m.detailScroll > 0 && m.detailScroll < len(lines) {
		lines = lines[m.detailScroll:]
	}

	// Truncate to fit
	maxLines := height - 3
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	content := strings.Join(lines, "\n")
	return borderStyle.Width(width - 2).Height(height - 2).Render(content)
}

func renderDiffLine(line types.DiffLine, maxWidth int) string {
	content := line.Content
	if len(content) > maxWidth-3 {
		content = content[:maxWidth-4] + "…"
	}

	switch line.Type {
	case types.DiffLineAdd:
		return diffAddStyle.Render("  +" + content)
	case types.DiffLineDelete:
		return diffDelStyle.Render("  -" + content)
	default:
		return diffContextStyle.Render("   " + content)
	}
}
