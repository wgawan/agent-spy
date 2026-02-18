package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wgawan/agent-spy/internal/types"
	"github.com/wgawan/agent-spy/internal/watcher"
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
