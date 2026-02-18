package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/wgawan/agent-spy/internal/types"
)

type Config struct {
	Path       string
	EventsChan chan types.FileEvent
	Debounce   time.Duration
	Filters    []string // glob patterns to exclude
}

type Watcher struct {
	config  Config
	fsw     *fsnotify.Watcher
	filter  *SmartFilter
	pending map[string]*pendingEvent
	mu      sync.Mutex
	done    chan struct{}
}

type pendingEvent struct {
	events []types.FileEvent
	timer  *time.Timer
}

func New(cfg Config) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if cfg.Debounce == 0 {
		cfg.Debounce = 500 * time.Millisecond
	}

	w := &Watcher{
		config:  cfg,
		fsw:     fsw,
		filter:  NewSmartFilter(cfg.Filters),
		pending: make(map[string]*pendingEvent),
		done:    make(chan struct{}),
	}

	// Walk directory and add all subdirectories
	err = filepath.Walk(cfg.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			// Skip filtered directories
			relPath, relErr := filepath.Rel(cfg.Path, path)
			if relErr == nil && w.filter.IsFiltered(relPath+"/") {
				return filepath.SkipDir
			}
			return fsw.Add(path)
		}
		return nil
	})
	if err != nil {
		fsw.Close()
		return nil, err
	}

	return w, nil
}

func (w *Watcher) Start() {
	for {
		select {
		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
		case <-w.done:
			return
		}
	}
}

func (w *Watcher) Close() {
	close(w.done)
	w.fsw.Close()
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	op := fsOpToType(event.Op)
	if op == -1 {
		return
	}

	// Check if this is a new directory being created
	if event.Op.Has(fsnotify.Create) {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			w.fsw.Add(event.Name)
			return // Don't emit events for directory creation
		}
	}

	// Make path relative
	relPath, err := filepath.Rel(w.config.Path, event.Name)
	if err != nil {
		relPath = event.Name
	}

	// Skip filtered paths
	if w.isFiltered(relPath) {
		return
	}

	fe := types.FileEvent{
		Path:      relPath,
		Op:        op,
		Timestamp: time.Now(),
	}

	w.debounce(fe)
}

func (w *Watcher) debounce(fe types.FileEvent) {
	w.mu.Lock()
	defer w.mu.Unlock()

	key := fe.Path

	if p, exists := w.pending[key]; exists {
		p.events = append(p.events, fe)
		p.timer.Reset(w.config.Debounce)
		return
	}

	p := &pendingEvent{
		events: []types.FileEvent{fe},
	}
	p.timer = time.AfterFunc(w.config.Debounce, func() {
		w.flush(key)
	})
	w.pending[key] = p
}

func (w *Watcher) flush(key string) {
	w.mu.Lock()
	p, exists := w.pending[key]
	if !exists {
		w.mu.Unlock()
		return
	}
	delete(w.pending, key)
	w.mu.Unlock()

	last := p.events[len(p.events)-1]

	// Determine the effective operation: CREATE takes priority
	// (e.g. CREATE + WRITE should remain CREATE)
	op := last.Op
	for _, ev := range p.events {
		if ev.Op == types.OpCreate {
			op = types.OpCreate
			break
		}
	}

	result := types.FileEvent{
		Path:      last.Path,
		Op:        op,
		Timestamp: last.Timestamp,
	}
	if len(p.events) > 1 {
		result.SubEvents = p.events
	}

	w.config.EventsChan <- result
}

func (w *Watcher) isFiltered(path string) bool {
	return w.filter.IsFiltered(path)
}

func fsOpToType(op fsnotify.Op) types.Operation {
	switch {
	case op.Has(fsnotify.Create):
		return types.OpCreate
	case op.Has(fsnotify.Write):
		return types.OpModify
	case op.Has(fsnotify.Remove):
		return types.OpDelete
	case op.Has(fsnotify.Rename):
		return types.OpRename
	default:
		return -1
	}
}
