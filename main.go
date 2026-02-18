package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	gitpkg "github.com/wally/agent-spy/internal/git"
	"github.com/wally/agent-spy/internal/logger"
	"github.com/wally/agent-spy/internal/tui"
	"github.com/wally/agent-spy/internal/types"
	"github.com/wally/agent-spy/internal/watcher"
)

type stringSlice []string

func (s *stringSlice) String() string { return fmt.Sprintf("%v", *s) }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	debounce := flag.Int("debounce", 500, "debounce interval in milliseconds")
	logFile := flag.String("log", "", "write events to log file")
	noGit := flag.Bool("no-git", false, "disable git integration")
	var filters stringSlice
	flag.Var(&filters, "filter", "additional exclude patterns (can be specified multiple times)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: agent-spy [flags] [path]\n\n")
		fmt.Fprintf(os.Stderr, "A live TUI for watching file changes in your project.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	watchPath := "."
	if flag.NArg() > 0 {
		watchPath = flag.Arg(0)
	}

	absPath, err := filepath.Abs(watchPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
		os.Exit(1)
	}

	// Verify path exists
	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a directory\n", absPath)
		os.Exit(1)
	}

	// Git setup
	var repo *gitpkg.Repo
	var gitBranch string
	var gitAvailable bool
	var diffFn func(string) (types.DiffResult, error)

	if !*noGit {
		repo, _ = gitpkg.Open(absPath)
		if repo != nil && repo.Available() {
			gitAvailable = true
			gitBranch = repo.Branch()
			diffFn = repo.Diff
		}
	}

	// Set up log file if requested
	var logWriter *os.File
	if *logFile != "" {
		logWriter, err = os.OpenFile(*logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
			os.Exit(1)
		}
		defer logWriter.Close()
	}

	// Create event channel
	events := make(chan types.FileEvent, 100)

	// Set up file watcher
	var extraFilters []string
	if gitAvailable {
		extraFilters = repo.IgnorePatterns()
	}
	extraFilters = append(extraFilters, filters...)

	w, err := watcher.New(watcher.Config{
		Path:       absPath,
		EventsChan: events,
		Debounce:   time.Duration(*debounce) * time.Millisecond,
		Filters:    extraFilters,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting watcher: %v\n", err)
		os.Exit(1)
	}
	defer w.Close()

	go w.Start()

	// If logging, wrap the events channel
	if logWriter != nil {
		l := logger.New(logWriter)
		loggedEvents := make(chan types.FileEvent, 100)
		go func() {
			for ev := range events {
				var stats *types.DiffStats
				if diffFn != nil {
					diff, _ := diffFn(ev.Path)
					if diff.Available {
						stats = &diff.Stats
					}
				}
				l.LogEvent(ev, stats)
				loggedEvents <- ev
			}
		}()
		events = loggedEvents
	}

	// Display path relative to home for nicer display
	displayPath := absPath
	if home, err := os.UserHomeDir(); err == nil {
		if rel, err := filepath.Rel(home, absPath); err == nil {
			displayPath = "~/" + rel
		}
	}

	// Start TUI
	model := tui.New(events, displayPath, gitBranch, gitAvailable, diffFn)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
