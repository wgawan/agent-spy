package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wally/agent-spy/internal/types"
)

type Model struct {
	events       []types.FileEvent
	eventsChan   chan types.FileEvent
	selected     int
	width        int
	height       int
	fullscreen   bool
	filterMode   bool
	filterText   string
	startTime    time.Time
	totalAdded   int
	totalDeleted int
	uniqueFiles  map[string]bool
	gitBranch    string
	gitAvailable bool
	watchPath    string
	diffFn       func(string) (types.DiffResult, error)
	currentDiff  types.DiffResult
	detailScroll int
	quitting     bool
}

type fileEventMsg types.FileEvent
type tickMsg time.Time

func New(eventsChan chan types.FileEvent, watchPath string, gitBranch string, gitAvailable bool, diffFn func(string) (types.DiffResult, error)) Model {
	return Model{
		events:       make([]types.FileEvent, 0),
		eventsChan:   eventsChan,
		uniqueFiles:  make(map[string]bool),
		startTime:    time.Now(),
		gitBranch:    gitBranch,
		gitAvailable: gitAvailable,
		watchPath:    watchPath,
		diffFn:       diffFn,
	}
}

func waitForEvent(ch chan types.FileEvent) tea.Cmd {
	return func() tea.Msg {
		ev := <-ch
		return fileEventMsg(ev)
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(waitForEvent(m.eventsChan), tick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case fileEventMsg:
		ev := types.FileEvent(msg)
		m.events = append([]types.FileEvent{ev}, m.events...) // prepend (newest first)
		m.uniqueFiles[ev.Path] = true
		// Update diff for selected event
		if m.selected == 0 && m.diffFn != nil {
			diff, _ := m.diffFn(ev.Path)
			m.currentDiff = diff
			m.totalAdded += diff.Stats.Added
			m.totalDeleted += diff.Stats.Deleted
		}
		return m, waitForEvent(m.eventsChan)
	case tickMsg:
		return m, tick()
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filterMode {
		switch msg.String() {
		case "enter", "esc":
			m.filterMode = false
			return m, nil
		case "backspace":
			if len(m.filterText) > 0 {
				m.filterText = m.filterText[:len(m.filterText)-1]
			}
			return m, nil
		default:
			if len(msg.String()) == 1 {
				m.filterText += msg.String()
			}
			return m, nil
		}
	}

	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.selected > 0 {
			m.selected--
			m.updateSelectedDiff()
			m.detailScroll = 0
		}
		return m, nil
	case "down", "j":
		if m.selected < len(m.events)-1 {
			m.selected++
			m.updateSelectedDiff()
			m.detailScroll = 0
		}
		return m, nil
	case "F":
		m.fullscreen = !m.fullscreen
		return m, nil
	case "esc":
		if m.fullscreen {
			m.fullscreen = false
		}
		return m, nil
	case "f":
		m.filterMode = true
		m.filterText = ""
		return m, nil
	case "c":
		m.events = nil
		m.selected = 0
		m.uniqueFiles = make(map[string]bool)
		m.totalAdded = 0
		m.totalDeleted = 0
		m.currentDiff = types.DiffResult{}
		return m, nil
	case "ctrl+d":
		m.detailScroll++
		return m, nil
	case "ctrl+u":
		if m.detailScroll > 0 {
			m.detailScroll--
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) updateSelectedDiff() {
	if m.selected >= 0 && m.selected < len(m.events) && m.diffFn != nil {
		diff, _ := m.diffFn(m.events[m.selected].Path)
		m.currentDiff = diff
	}
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 {
		return "Initializing..."
	}
	return m.renderLayout()
}
