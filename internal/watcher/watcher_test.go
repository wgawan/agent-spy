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
