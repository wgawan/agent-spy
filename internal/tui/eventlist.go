package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/wgawan/agent-spy/internal/types"
)

func (m Model) renderEventList(width, height int) string {
	if len(m.events) == 0 {
		content := normalStyle.Render("  Watching for changes...")
		return borderStyle.Width(width - 2).Height(height - 2).Render(content)
	}

	var lines []string
	header := headerStyle.Render(" Events")
	lines = append(lines, header)

	filteredEvents := m.filteredEvents()

	for i, ev := range filteredEvents {
		if i >= height-3 { // leave room for header and border
			break
		}
		if i == m.selected {
			line := formatEventLine(ev, width-6)
			line = selectedStyle.Width(width - 4).Render("▶ " + line)
			lines = append(lines, line)
		} else {
			line := formatEventLine(ev, width-6)
			line = normalStyle.Width(width - 4).Render("  " + line)
			lines = append(lines, line)
		}
	}

	content := strings.Join(lines, "\n")
	return borderStyle.Width(width - 2).Height(height - 2).Render(content)
}

func (m Model) filteredEvents() []types.FileEvent {
	if m.filterText == "" {
		return m.events
	}
	var filtered []types.FileEvent
	for _, ev := range m.events {
		if strings.Contains(ev.Path, m.filterText) {
			filtered = append(filtered, ev)
		}
	}
	return filtered
}

func formatEventLine(ev types.FileEvent, maxWidth int) string {
	ts := ev.Timestamp.Format("15:04:05")
	sym := ev.Op.Symbol()
	path := ev.Path

	suffix := ""
	if ev.IsDebounced() {
		suffix = fmt.Sprintf("(x%d)", ev.ChangeCount())
	}

	line := fmt.Sprintf(" %s %s %s %s", ts, sym, path, suffix)
	if len(line) > maxWidth {
		line = line[:maxWidth-1] + "…"
	}
	return line
}

// Ensure lipgloss is used (referenced in renderEventList)
var _ = lipgloss.Width
