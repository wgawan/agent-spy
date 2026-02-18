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
