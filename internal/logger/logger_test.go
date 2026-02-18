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
