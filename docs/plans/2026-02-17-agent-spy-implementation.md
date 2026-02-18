# agent-spy Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a live TUI that watches a directory for file changes and displays diffs, stats, and an event stream in real-time.

**Architecture:** Three packages (`watcher`, `git`, `tui`) coordinated by `main.go`. The watcher sends `FileEvent` structs over a channel to the bubbletea TUI, which calls the git package on demand for diffs. The git package is optional and gracefully degrades when not in a git repo.

**Tech Stack:** Go, bubbletea/lipgloss/bubbles (Charm), fsnotify, go-git/v5

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `internal/watcher/watcher.go`
- Create: `internal/git/git.go`
- Create: `internal/tui/tui.go`
- Create: `.gitignore`

**Step 1: Initialize Go module**

Run: `go mod init github.com/wally/agent-spy`
Expected: `go.mod` created

**Step 2: Create directory structure**

Run: `mkdir -p internal/watcher internal/git internal/tui`

**Step 3: Create .gitignore**

```gitignore
agent-spy
*.exe
dist/
```

**Step 4: Create placeholder main.go**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: agent-spy [path]")
		os.Exit(1)
	}
	fmt.Printf("Watching: %s\n", os.Args[1])
}
```

**Step 5: Verify it builds**

Run: `go build -o agent-spy . && ./agent-spy .`
Expected: `Watching: .`

**Step 6: Commit**

```bash
git add go.mod main.go .gitignore internal/
git commit -m "feat: project scaffolding"
```

---

### Task 2: Core Types

**Files:**
- Create: `internal/types/types.go`
- Create: `internal/types/types_test.go`

**Step 1: Write the test**

```go
package types

import (
	"testing"
	"time"
)

func TestOperationString(t *testing.T) {
	tests := []struct {
		op   Operation
		want string
	}{
		{OpCreate, "CREATE"},
		{OpModify, "MODIFY"},
		{OpDelete, "DELETE"},
		{OpRename, "RENAME"},
	}
	for _, tt := range tests {
		if got := tt.op.String(); got != tt.want {
			t.Errorf("Operation(%d).String() = %q, want %q", tt.op, got, tt.want)
		}
	}
}

func TestOperationSymbol(t *testing.T) {
	tests := []struct {
		op   Operation
		want string
	}{
		{OpCreate, "+"},
		{OpModify, "M"},
		{OpDelete, "D"},
		{OpRename, "R"},
	}
	for _, tt := range tests {
		if got := tt.op.Symbol(); got != tt.want {
			t.Errorf("Operation(%d).Symbol() = %q, want %q", tt.op, got, tt.want)
		}
	}
}

func TestFileEventDebounced(t *testing.T) {
	now := time.Now()
	ev := FileEvent{
		Path:      "src/app.go",
		Op:        OpModify,
		Timestamp: now,
		SubEvents: []FileEvent{
			{Path: "src/app.go", Op: OpModify, Timestamp: now.Add(-400 * time.Millisecond)},
			{Path: "src/app.go", Op: OpModify, Timestamp: now.Add(-200 * time.Millisecond)},
			{Path: "src/app.go", Op: OpModify, Timestamp: now},
		},
	}
	if !ev.IsDebounced() {
		t.Error("expected event with 3 sub-events to be debounced")
	}
	if ev.ChangeCount() != 3 {
		t.Errorf("expected ChangeCount=3, got %d", ev.ChangeCount())
	}

	single := FileEvent{Path: "README.md", Op: OpModify, Timestamp: now}
	if single.IsDebounced() {
		t.Error("expected event with no sub-events to not be debounced")
	}
	if single.ChangeCount() != 1 {
		t.Errorf("expected ChangeCount=1, got %d", single.ChangeCount())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/types/ -v`
Expected: FAIL (types not defined)

**Step 3: Write implementation**

```go
package types

import "time"

type Operation int

const (
	OpCreate Operation = iota
	OpModify
	OpDelete
	OpRename
)

func (o Operation) String() string {
	switch o {
	case OpCreate:
		return "CREATE"
	case OpModify:
		return "MODIFY"
	case OpDelete:
		return "DELETE"
	case OpRename:
		return "RENAME"
	default:
		return "UNKNOWN"
	}
}

func (o Operation) Symbol() string {
	switch o {
	case OpCreate:
		return "+"
	case OpModify:
		return "M"
	case OpDelete:
		return "D"
	case OpRename:
		return "R"
	default:
		return "?"
	}
}

type FileEvent struct {
	Path      string
	Op        Operation
	Timestamp time.Time
	SubEvents []FileEvent
}

func (e FileEvent) IsDebounced() bool {
	return len(e.SubEvents) > 1
}

func (e FileEvent) ChangeCount() int {
	if len(e.SubEvents) == 0 {
		return 1
	}
	return len(e.SubEvents)
}

type DiffHunk struct {
	Header   string
	Lines    []DiffLine
}

type DiffLine struct {
	Content string
	Type    DiffLineType
}

type DiffLineType int

const (
	DiffLineContext DiffLineType = iota
	DiffLineAdd
	DiffLineDelete
)

type DiffStats struct {
	Added   int
	Deleted int
}

type DiffResult struct {
	Available bool
	Hunks     []DiffHunk
	Stats     DiffStats
	Error     string
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/types/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/types/
git commit -m "feat: add core types (FileEvent, DiffResult, Operation)"
```

---

### Task 3: File Watcher - Basic Watching

**Files:**
- Create: `internal/watcher/watcher.go`
- Create: `internal/watcher/watcher_test.go`

**Step 1: Install fsnotify**

Run: `go get github.com/fsnotify/fsnotify`

**Step 2: Write the test**

```go
package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wally/agent-spy/internal/types"
)

func TestWatcherDetectsCreate(t *testing.T) {
	dir := t.TempDir()
	events := make(chan types.FileEvent, 10)

	w, err := New(Config{
		Path:       dir,
		EventsChan: events,
		Debounce:   50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	go w.Start()

	// Give watcher time to initialize
	time.Sleep(100 * time.Millisecond)

	// Create a file
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	select {
	case ev := <-events:
		if ev.Op != types.OpCreate {
			t.Errorf("expected OpCreate, got %v", ev.Op)
		}
		if filepath.Base(ev.Path) != "test.txt" {
			t.Errorf("expected test.txt, got %s", ev.Path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for create event")
	}
}

func TestWatcherDetectsModify(t *testing.T) {
	dir := t.TempDir()
	events := make(chan types.FileEvent, 10)

	// Pre-create the file
	testFile := filepath.Join(dir, "existing.txt")
	os.WriteFile(testFile, []byte("original"), 0644)

	w, err := New(Config{
		Path:       dir,
		EventsChan: events,
		Debounce:   50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	go w.Start()
	time.Sleep(100 * time.Millisecond)

	// Modify the file
	os.WriteFile(testFile, []byte("modified"), 0644)

	select {
	case ev := <-events:
		if ev.Op != types.OpModify {
			t.Errorf("expected OpModify, got %v", ev.Op)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for modify event")
	}
}

func TestWatcherDetectsDelete(t *testing.T) {
	dir := t.TempDir()
	events := make(chan types.FileEvent, 10)

	testFile := filepath.Join(dir, "todelete.txt")
	os.WriteFile(testFile, []byte("bye"), 0644)

	w, err := New(Config{
		Path:       dir,
		EventsChan: events,
		Debounce:   50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	go w.Start()
	time.Sleep(100 * time.Millisecond)

	os.Remove(testFile)

	select {
	case ev := <-events:
		if ev.Op != types.OpDelete {
			t.Errorf("expected OpDelete, got %v", ev.Op)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for delete event")
	}
}

func TestWatcherRecursive(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "subdir")
	os.MkdirAll(subDir, 0755)

	events := make(chan types.FileEvent, 10)

	w, err := New(Config{
		Path:       dir,
		EventsChan: events,
		Debounce:   50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer w.Close()

	go w.Start()
	time.Sleep(100 * time.Millisecond)

	// Create file in subdirectory
	testFile := filepath.Join(subDir, "nested.txt")
	os.WriteFile(testFile, []byte("nested"), 0644)

	select {
	case ev := <-events:
		if filepath.Base(ev.Path) != "nested.txt" {
			t.Errorf("expected nested.txt, got %s", ev.Path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event in subdirectory")
	}
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./internal/watcher/ -v`
Expected: FAIL (watcher.New not defined)

**Step 4: Write implementation**

```go
package watcher

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/wally/agent-spy/internal/types"
)

type Config struct {
	Path       string
	EventsChan chan types.FileEvent
	Debounce   time.Duration
	Filters    []string // glob patterns to exclude
}

type Watcher struct {
	config    Config
	fsw       *fsnotify.Watcher
	pending   map[string]*pendingEvent
	mu        sync.Mutex
	done      chan struct{}
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
			base := filepath.Base(path)
			if base == ".git" {
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
	result := types.FileEvent{
		Path:      last.Path,
		Op:        last.Op,
		Timestamp: last.Timestamp,
	}
	if len(p.events) > 1 {
		result.SubEvents = p.events
	}

	w.config.EventsChan <- result
}

func (w *Watcher) isFiltered(path string) bool {
	for _, pattern := range w.config.Filters {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
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
```

**Step 5: Run tests**

Run: `go test ./internal/watcher/ -v -timeout 30s`
Expected: PASS (all 4 tests)

**Step 6: Commit**

```bash
git add internal/watcher/ go.mod go.sum
git commit -m "feat: add file watcher with debouncing and recursive watching"
```

---

### Task 4: Smart Default Filters

**Files:**
- Create: `internal/watcher/filter.go`
- Create: `internal/watcher/filter_test.go`

**Step 1: Write the test**

```go
package watcher

import "testing"

func TestSmartFilter(t *testing.T) {
	f := NewSmartFilter(nil)

	tests := []struct {
		path    string
		filtered bool
	}{
		{".git/config", true},
		{"node_modules/foo/bar.js", true},
		{"vendor/lib/thing.go", true},
		{"__pycache__/mod.pyc", true},
		{".DS_Store", true},
		{"Thumbs.db", true},
		{"build/output.js", true},
		{"dist/bundle.js", true},
		{".next/cache/foo", true},
		{"target/debug/bin", true},
		{".venv/lib/python/site.py", true},
		{"package-lock.json", true},
		{"yarn.lock", true},
		{"pnpm-lock.yaml", true},
		{"foo.pyc", true},
		{"bar.o", true},
		{"Baz.class", true},
		// Should NOT be filtered:
		{"src/app.go", false},
		{"README.md", false},
		{"internal/types/types.go", false},
		{"package.json", false},
		{"Makefile", false},
	}

	for _, tt := range tests {
		if got := f.IsFiltered(tt.path); got != tt.filtered {
			t.Errorf("IsFiltered(%q) = %v, want %v", tt.path, got, tt.filtered)
		}
	}
}

func TestSmartFilterWithExtra(t *testing.T) {
	f := NewSmartFilter([]string{"*.log", "tmp/"})

	if !f.IsFiltered("debug.log") {
		t.Error("expected debug.log to be filtered")
	}
	if !f.IsFiltered("tmp/cache.dat") {
		t.Error("expected tmp/cache.dat to be filtered")
	}
	if f.IsFiltered("src/main.go") {
		t.Error("expected src/main.go to NOT be filtered")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/watcher/ -run TestSmartFilter -v`
Expected: FAIL

**Step 3: Write implementation**

```go
package watcher

import (
	"path/filepath"
	"strings"
)

var defaultFilteredDirs = []string{
	".git",
	"node_modules",
	"vendor",
	".venv",
	"__pycache__",
	"build",
	"dist",
	".next",
	".nuxt",
	"target",
}

var defaultFilteredFiles = []string{
	".DS_Store",
	"Thumbs.db",
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
}

var defaultFilteredExts = []string{
	".lock",
	".pyc",
	".o",
	".class",
}

type SmartFilter struct {
	filteredDirs  []string
	filteredFiles []string
	filteredExts  []string
	extraPatterns []string
}

func NewSmartFilter(extraPatterns []string) *SmartFilter {
	return &SmartFilter{
		filteredDirs:  defaultFilteredDirs,
		filteredFiles: defaultFilteredFiles,
		filteredExts:  defaultFilteredExts,
		extraPatterns: extraPatterns,
	}
}

func (f *SmartFilter) IsFiltered(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")

	// Check each path component against filtered directories
	for _, part := range parts {
		for _, dir := range f.filteredDirs {
			if part == dir {
				return true
			}
		}
	}

	// Check filename against filtered files
	base := filepath.Base(path)
	for _, name := range f.filteredFiles {
		if base == name {
			return true
		}
	}

	// Check extension
	ext := filepath.Ext(base)
	for _, filteredExt := range f.filteredExts {
		if ext == filteredExt {
			return true
		}
	}

	// Check extra patterns
	for _, pattern := range f.extraPatterns {
		// Directory pattern (ends with /)
		if strings.HasSuffix(pattern, "/") {
			dirName := strings.TrimSuffix(pattern, "/")
			for _, part := range parts {
				if part == dirName {
					return true
				}
			}
			continue
		}
		// Glob pattern
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}

	return false
}
```

**Step 4: Run tests**

Run: `go test ./internal/watcher/ -run TestSmartFilter -v`
Expected: PASS

**Step 5: Integrate SmartFilter into Watcher**

Update `watcher.go` to use `SmartFilter` instead of the simple `isFiltered` method. Replace the `isFiltered` method:

```go
// In the Watcher struct, add:
//   filter *SmartFilter

// In New(), add:
//   w.filter = NewSmartFilter(cfg.Filters)

// Replace isFiltered method with:
func (w *Watcher) isFiltered(path string) bool {
    return w.filter.IsFiltered(path)
}
```

Also update the `filepath.Walk` in `New()` to use the filter for skipping directories.

**Step 6: Run all watcher tests**

Run: `go test ./internal/watcher/ -v -timeout 30s`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/watcher/
git commit -m "feat: add smart default filters for common noise files"
```

---

### Task 5: Git Integration - Repository Detection & Branch

**Files:**
- Create: `internal/git/git.go`
- Create: `internal/git/git_test.go`

**Step 1: Install go-git**

Run: `go get github.com/go-git/go-git/v5`

**Step 2: Write the test**

```go
package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	cmd.Run()
	// Create initial commit
	testFile := filepath.Join(dir, "README.md")
	os.WriteFile(testFile, []byte("# Test\n"), 0644)
	cmd = exec.Command("git", "-C", dir, "add", ".")
	cmd.Run()
	cmd = exec.Command("git", "-C", dir, "commit", "-m", "init")
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
	cmd.Run()
	return dir
}

func TestRepoDetection(t *testing.T) {
	dir := initTestRepo(t)
	r, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	if !r.Available() {
		t.Error("expected repo to be available")
	}
}

func TestRepoDetectionNonRepo(t *testing.T) {
	dir := t.TempDir()
	r, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	if r.Available() {
		t.Error("expected repo to NOT be available for non-git dir")
	}
}

func TestBranch(t *testing.T) {
	dir := initTestRepo(t)
	r, _ := Open(dir)
	branch := r.Branch()
	if branch != "master" && branch != "main" {
		t.Errorf("expected master or main, got %q", branch)
	}
}

func TestBranchNonRepo(t *testing.T) {
	dir := t.TempDir()
	r, _ := Open(dir)
	if r.Branch() != "" {
		t.Error("expected empty branch for non-repo")
	}
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./internal/git/ -v`
Expected: FAIL

**Step 4: Write implementation**

```go
package git

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type Repo struct {
	repo *git.Repository
	path string
}

func Open(path string) (*Repo, error) {
	r := &Repo{path: path}
	repo, err := git.PlainOpen(path)
	if err != nil {
		// Not a git repo - that's fine, gracefully degrade
		return r, nil
	}
	r.repo = repo
	return r, nil
}

func (r *Repo) Available() bool {
	return r.repo != nil
}

func (r *Repo) Branch() string {
	if r.repo == nil {
		return ""
	}
	ref, err := r.repo.Head()
	if err != nil {
		return ""
	}
	if ref.Name().IsBranch() {
		return ref.Name().Short()
	}
	// Detached HEAD - return short hash
	return ref.Hash().String()[:7]
}

func (r *Repo) Path() string {
	return r.path
}
```

**Step 5: Run tests**

Run: `go test ./internal/git/ -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/git/ go.mod go.sum
git commit -m "feat: add git repo detection and branch info"
```

---

### Task 6: Git Integration - Diff Generation

**Files:**
- Modify: `internal/git/git.go`
- Modify: `internal/git/git_test.go`

**Step 1: Write the test**

Append to `git_test.go`:

```go
func TestDiffModifiedFile(t *testing.T) {
	dir := initTestRepo(t)
	r, _ := Open(dir)

	// Modify the file
	testFile := filepath.Join(dir, "README.md")
	os.WriteFile(testFile, []byte("# Test\n\nNew content\n"), 0644)

	diff, err := r.Diff("README.md")
	if err != nil {
		t.Fatalf("Diff() error: %v", err)
	}
	if !diff.Available {
		t.Error("expected diff to be available")
	}
	if diff.Stats.Added == 0 {
		t.Error("expected added lines > 0")
	}
}

func TestDiffNewFile(t *testing.T) {
	dir := initTestRepo(t)
	r, _ := Open(dir)

	// Create a new file
	os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main\n\nfunc hello() {}\n"), 0644)

	diff, err := r.Diff("new.go")
	if err != nil {
		t.Fatalf("Diff() error: %v", err)
	}
	if !diff.Available {
		t.Error("expected diff to be available")
	}
	if diff.Stats.Added != 3 {
		t.Errorf("expected 3 added lines, got %d", diff.Stats.Added)
	}
}

func TestDiffNonRepo(t *testing.T) {
	dir := t.TempDir()
	r, _ := Open(dir)
	diff, err := r.Diff("anything.txt")
	if err != nil {
		t.Fatalf("Diff() error: %v", err)
	}
	if diff.Available {
		t.Error("expected diff to NOT be available for non-repo")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/git/ -run TestDiff -v`
Expected: FAIL

**Step 3: Write implementation**

Add to `git.go`:

```go
import (
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/wally/agent-spy/internal/types"
)

func (r *Repo) Diff(relPath string) (types.DiffResult, error) {
	if r.repo == nil {
		return types.DiffResult{Available: false, Error: "not a git repository"}, nil
	}

	wt, err := r.repo.Worktree()
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}

	status, err := wt.Status()
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}

	fileStatus := status.File(relPath)
	if fileStatus.Worktree == git.Unmodified && fileStatus.Staging == git.Unmodified {
		return types.DiffResult{Available: false, Error: "file not modified"}, nil
	}

	// Get HEAD tree
	ref, err := r.repo.Head()
	if err != nil {
		// No commits yet - entire file is "added"
		return r.diffNewFile(relPath)
	}

	commit, err := r.repo.CommitObject(ref.Hash())
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}

	tree, err := commit.Tree()
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}

	// Try to get the file from HEAD
	_, err = tree.File(relPath)
	if err != nil {
		// File doesn't exist in HEAD - it's new
		return r.diffNewFile(relPath)
	}

	// Get patch between HEAD and worktree
	// Use go-git's diff functionality
	patch, err := r.getWorkingDirPatch(commit, tree, relPath)
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}

	return patch, nil
}

func (r *Repo) diffNewFile(relPath string) (types.DiffResult, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}

	fs := wt.Filesystem
	f, err := fs.Open(relPath)
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}
	defer f.Close()

	buf := make([]byte, 32*1024)
	n, _ := f.Read(buf)
	content := string(buf[:n])
	lines := strings.Split(content, "\n")

	var diffLines []types.DiffLine
	for _, line := range lines {
		if line != "" {
			diffLines = append(diffLines, types.DiffLine{
				Content: line,
				Type:    types.DiffLineAdd,
			})
		}
	}

	return types.DiffResult{
		Available: true,
		Hunks: []types.DiffHunk{
			{Header: "@@ -0,0 +1," + fmt.Sprintf("%d", len(diffLines)) + " @@", Lines: diffLines},
		},
		Stats: types.DiffStats{Added: len(diffLines)},
	}, nil
}

func (r *Repo) getWorkingDirPatch(commit *object.Commit, headTree *object.Tree, relPath string) (types.DiffResult, error) {
	// Read current file from working directory
	wt, _ := r.repo.Worktree()
	fs := wt.Filesystem
	f, err := fs.Open(relPath)
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}
	defer f.Close()

	buf := make([]byte, 64*1024)
	n, _ := f.Read(buf)
	newContent := string(buf[:n])

	// Read old file from HEAD
	headFile, err := headTree.File(relPath)
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}
	oldContent, err := headFile.Contents()
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}

	// Simple line-by-line diff
	return computeSimpleDiff(oldContent, newContent), nil
}

func computeSimpleDiff(old, new string) types.DiffResult {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	var hunks []types.DiffHunk
	var lines []types.DiffLine
	added, deleted := 0, 0

	// Simple diff: show removed lines then added lines
	// (A proper implementation would use Myers diff algorithm,
	// but this is sufficient for the initial version)
	oldSet := make(map[string]bool)
	for _, l := range oldLines {
		oldSet[l] = true
	}
	newSet := make(map[string]bool)
	for _, l := range newLines {
		newSet[l] = true
	}

	for _, l := range oldLines {
		if !newSet[l] && l != "" {
			lines = append(lines, types.DiffLine{Content: l, Type: types.DiffLineDelete})
			deleted++
		} else if l != "" {
			lines = append(lines, types.DiffLine{Content: l, Type: types.DiffLineContext})
		}
	}
	for _, l := range newLines {
		if !oldSet[l] && l != "" {
			lines = append(lines, types.DiffLine{Content: l, Type: types.DiffLineAdd})
			added++
		}
	}

	if len(lines) > 0 {
		hunks = append(hunks, types.DiffHunk{
			Header: fmt.Sprintf("@@ -%d +%d @@", len(oldLines), len(newLines)),
			Lines:  lines,
		})
	}

	return types.DiffResult{
		Available: true,
		Hunks:     hunks,
		Stats:     types.DiffStats{Added: added, Deleted: deleted},
	}
}
```

Note: add `"fmt"` to the imports.

**Step 4: Run tests**

Run: `go test ./internal/git/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/git/
git commit -m "feat: add git diff generation for modified and new files"
```

---

### Task 7: Git Integration - Gitignore Patterns

**Files:**
- Modify: `internal/git/git.go`
- Modify: `internal/git/git_test.go`

**Step 1: Write the test**

Append to `git_test.go`:

```go
func TestGitignorePatterns(t *testing.T) {
	dir := initTestRepo(t)

	// Create .gitignore
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\ntmp/\n"), 0644)

	r, _ := Open(dir)
	patterns := r.IgnorePatterns()

	if len(patterns) == 0 {
		t.Fatal("expected ignore patterns")
	}

	// Should contain *.log and tmp/
	found := map[string]bool{}
	for _, p := range patterns {
		found[p] = true
	}
	if !found["*.log"] {
		t.Error("expected *.log in patterns")
	}
	if !found["tmp/"] {
		t.Error("expected tmp/ in patterns")
	}
}

func TestGitignorePatternsNonRepo(t *testing.T) {
	dir := t.TempDir()
	r, _ := Open(dir)
	patterns := r.IgnorePatterns()
	if len(patterns) != 0 {
		t.Errorf("expected no patterns for non-repo, got %v", patterns)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/git/ -run TestGitignore -v`
Expected: FAIL

**Step 3: Write implementation**

Add to `git.go`:

```go
import "bufio"

func (r *Repo) IgnorePatterns() []string {
	if r.repo == nil {
		return nil
	}

	wt, err := r.repo.Worktree()
	if err != nil {
		return nil
	}

	f, err := wt.Filesystem.Open(".gitignore")
	if err != nil {
		return nil
	}
	defer f.Close()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}
```

**Step 4: Run tests**

Run: `go test ./internal/git/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/git/
git commit -m "feat: add gitignore pattern extraction"
```

---

### Task 8: TUI - Core Model & Stats Bar

**Files:**
- Create: `internal/tui/model.go`
- Create: `internal/tui/statsbar.go`
- Create: `internal/tui/styles.go`

**Step 1: Install charm dependencies**

Run: `go get github.com/charmbracelet/bubbletea github.com/charmbracelet/lipgloss github.com/charmbracelet/bubbles`

**Step 2: Create styles**

```go
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
```

**Step 3: Create the core model**

```go
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
		events:      make([]types.FileEvent, 0),
		eventsChan:  eventsChan,
		uniqueFiles: make(map[string]bool),
		startTime:   time.Now(),
		gitBranch:   gitBranch,
		gitAvailable: gitAvailable,
		watchPath:   watchPath,
		diffFn:      diffFn,
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
		// Scroll detail down
		m.detailScroll++
		return m, nil
	case "ctrl+u":
		// Scroll detail up
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
```

**Step 4: Create stats bar rendering**

```go
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
```

**Step 5: Verify it compiles**

Run: `go build ./internal/tui/`
Expected: Success (may need to stub renderLayout)

**Step 6: Commit**

```bash
git add internal/tui/ go.mod go.sum
git commit -m "feat: add TUI core model, styles, and stats bar"
```

---

### Task 9: TUI - Event List & Detail Pane

**Files:**
- Create: `internal/tui/eventlist.go`
- Create: `internal/tui/detail.go`
- Create: `internal/tui/layout.go`

**Step 1: Create event list rendering**

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/wally/agent-spy/internal/types"
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
		line := formatEventLine(ev, width-4)
		if i == m.selected {
			line = selectedStyle.Width(width - 4).Render(line)
		} else {
			line = normalStyle.Width(width - 4).Render(line)
		}
		lines = append(lines, line)
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
```

**Step 2: Create detail pane rendering**

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/wally/agent-spy/internal/types"
)

func (m Model) renderDetail(width, height int) string {
	if len(m.events) == 0 {
		content := normalStyle.Render("  Select an event to view details")
		return borderStyle.Width(width - 2).Height(height - 2).Render(content)
	}

	var lines []string
	header := headerStyle.Render(" Detail")
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
```

**Step 3: Create layout rendering**

```go
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
```

**Step 4: Verify it compiles**

Run: `go build ./internal/tui/`
Expected: Success

**Step 5: Commit**

```bash
git add internal/tui/
git commit -m "feat: add TUI event list, detail pane, and layout"
```

---

### Task 10: Main Wiring - Connect Everything

**Files:**
- Modify: `main.go`

**Step 1: Write the full main.go**

```go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	gitpkg "github.com/wally/agent-spy/internal/git"
	"github.com/wally/agent-spy/internal/tui"
	"github.com/wally/agent-spy/internal/types"
	"github.com/wally/agent-spy/internal/watcher"
)

func main() {
	debounce := flag.Int("debounce", 500, "debounce interval in milliseconds")
	logFile := flag.String("log", "", "write events to log file")
	noGit := flag.Bool("no-git", false, "disable git integration")
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
		loggedEvents := make(chan types.FileEvent, 100)
		go func() {
			for ev := range events {
				// Write to log
				stats := ""
				if diffFn != nil {
					diff, _ := diffFn(ev.Path)
					if diff.Available {
						stats = fmt.Sprintf(" +%d -%d", diff.Stats.Added, diff.Stats.Deleted)
					}
				}
				fmt.Fprintf(logWriter, "%s %s %s%s\n",
					ev.Timestamp.Format(time.RFC3339),
					ev.Op.String(),
					ev.Path,
					stats,
				)
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
```

**Step 2: Verify it builds**

Run: `go build -o agent-spy .`
Expected: Binary created

**Step 3: Quick smoke test**

Run: `./agent-spy . &` (background it, then create a test file, then fg and quit)
Expected: TUI launches, shows the stats bar, event list area

**Step 4: Commit**

```bash
git add main.go go.mod go.sum
git commit -m "feat: wire up main - connect watcher, git, and TUI"
```

---

### Task 11: Log File Output

**Files:**
- Create: `internal/logger/logger.go`
- Create: `internal/logger/logger_test.go`

**Step 1: Write the test**

```go
package logger

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wally/agent-spy/internal/types"
)

func TestLoggerWritesEvents(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "agent-spy-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	l := New(f)

	ev := types.FileEvent{
		Path:      "src/app.go",
		Op:        types.OpModify,
		Timestamp: time.Date(2026, 2, 17, 14, 3, 2, 0, time.UTC),
	}
	stats := types.DiffStats{Added: 12, Deleted: 3}

	l.LogEvent(ev, &stats)

	// Read back
	content, _ := os.ReadFile(f.Name())
	line := strings.TrimSpace(string(content))

	expected := "2026-02-17T14:03:02Z MODIFY src/app.go +12 -3"
	if line != expected {
		t.Errorf("got %q, want %q", line, expected)
	}
}

func TestLoggerWithoutStats(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "agent-spy-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	l := New(f)

	ev := types.FileEvent{
		Path:      "README.md",
		Op:        types.OpCreate,
		Timestamp: time.Date(2026, 2, 17, 14, 3, 1, 0, time.UTC),
	}

	l.LogEvent(ev, nil)

	content, _ := os.ReadFile(f.Name())
	line := strings.TrimSpace(string(content))

	expected := "2026-02-17T14:03:01Z CREATE README.md"
	if line != expected {
		t.Errorf("got %q, want %q", line, expected)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/logger/ -v`
Expected: FAIL

**Step 3: Write implementation**

```go
package logger

import (
	"fmt"
	"io"
	"time"

	"github.com/wally/agent-spy/internal/types"
)

type Logger struct {
	w io.Writer
}

func New(w io.Writer) *Logger {
	return &Logger{w: w}
}

func (l *Logger) LogEvent(ev types.FileEvent, stats *types.DiffStats) {
	line := fmt.Sprintf("%s %s %s",
		ev.Timestamp.Format(time.RFC3339),
		ev.Op.String(),
		ev.Path,
	)
	if stats != nil {
		line += fmt.Sprintf(" +%d -%d", stats.Added, stats.Deleted)
	}
	fmt.Fprintln(l.w, line)
}
```

**Step 4: Run tests**

Run: `go test ./internal/logger/ -v`
Expected: PASS

**Step 5: Integrate logger into main.go**

Replace the inline logging in `main.go` with the `logger` package.

**Step 6: Commit**

```bash
git add internal/logger/ main.go
git commit -m "feat: add structured event logger for --log flag"
```

---

### Task 12: CLI Flag Parsing with --filter

**Files:**
- Modify: `main.go`

**Step 1: Add --filter flag support**

Add a custom flag type that collects multiple `--filter` values:

```go
type stringSlice []string

func (s *stringSlice) String() string { return fmt.Sprintf("%v", *s) }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}
```

Register: `var filters stringSlice` and `flag.Var(&filters, "filter", "additional exclude patterns (can be specified multiple times)")`

Pass `filters` to the watcher config alongside gitignore patterns.

**Step 2: Verify build**

Run: `go build -o agent-spy . && ./agent-spy --help`
Expected: Shows all flags including --filter

**Step 3: Commit**

```bash
git add main.go
git commit -m "feat: add --filter flag for additional exclude patterns"
```

---

### Task 13: Integration Test - End to End

**Files:**
- Create: `integration_test.go`

**Step 1: Write integration test**

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wally/agent-spy/internal/types"
	"github.com/wally/agent-spy/internal/watcher"
)

func TestEndToEndWatchAndEvent(t *testing.T) {
	dir := t.TempDir()
	events := make(chan types.FileEvent, 10)

	w, err := watcher.New(watcher.Config{
		Path:       dir,
		EventsChan: events,
		Debounce:   100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	go w.Start()
	time.Sleep(200 * time.Millisecond)

	// Simulate agent creating a file
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	select {
	case ev := <-events:
		if ev.Op != types.OpCreate {
			t.Errorf("expected CREATE, got %v", ev.Op)
		}
		if ev.Path != "main.go" {
			t.Errorf("expected main.go, got %s", ev.Path)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Simulate agent modifying the file
	time.Sleep(200 * time.Millisecond)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)

	select {
	case ev := <-events:
		if ev.Op != types.OpModify {
			t.Errorf("expected MODIFY, got %v", ev.Op)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for modify event")
	}
}

func TestSmartFiltersInAction(t *testing.T) {
	dir := t.TempDir()
	events := make(chan types.FileEvent, 10)

	// Create node_modules before starting watcher
	os.MkdirAll(filepath.Join(dir, "node_modules", "foo"), 0755)

	w, err := watcher.New(watcher.Config{
		Path:       dir,
		EventsChan: events,
		Debounce:   100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	go w.Start()
	time.Sleep(200 * time.Millisecond)

	// Write to node_modules (should be filtered)
	os.WriteFile(filepath.Join(dir, "node_modules", "foo", "index.js"), []byte("module.exports = {}"), 0644)

	// Write to src (should NOT be filtered)
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	time.Sleep(100 * time.Millisecond)
	os.WriteFile(filepath.Join(dir, "src", "app.go"), []byte("package main\n"), 0644)

	select {
	case ev := <-events:
		if filepath.Base(ev.Path) == "index.js" {
			t.Error("node_modules file should have been filtered")
		}
		// Should be app.go or src directory
	case <-time.After(3 * time.Second):
		t.Fatal("timeout - expected src/app.go event")
	}
}
```

**Step 2: Run integration tests**

Run: `go test -v -timeout 30s -run TestEndToEnd`
Expected: PASS

Run: `go test -v -timeout 30s -run TestSmartFilters`
Expected: PASS

**Step 3: Run all tests**

Run: `go test ./... -v -timeout 60s`
Expected: All PASS

**Step 4: Commit**

```bash
git add integration_test.go
git commit -m "test: add end-to-end integration tests"
```

---

### Task 14: Final Build & Polish

**Files:**
- Modify: `main.go` (version flag)
- Create: `Makefile`

**Step 1: Add version flag**

Add `version := flag.Bool("version", false, "print version")` and handle it:

```go
if *version {
    fmt.Println("agent-spy v0.1.0")
    os.Exit(0)
}
```

**Step 2: Create Makefile**

```makefile
.PHONY: build test clean

build:
	go build -o agent-spy .

test:
	go test ./... -v -timeout 60s

clean:
	rm -f agent-spy

install:
	go install .
```

**Step 3: Full build and test**

Run: `make clean && make build && make test`
Expected: All pass, binary created

**Step 4: Quick manual test**

Run: `./agent-spy --version`
Expected: `agent-spy v0.1.0`

**Step 5: Commit**

```bash
git add main.go Makefile
git commit -m "feat: add version flag and Makefile"
```

---

### Task 15: Final Review & Tag

**Step 1: Run full test suite**

Run: `go test ./... -v -timeout 60s -race`
Expected: PASS with no race conditions

**Step 2: Build release binary**

Run: `go build -o agent-spy .`
Expected: Binary created

**Step 3: Tag release**

```bash
git tag v0.1.0
```
