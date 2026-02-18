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
	Header string
	Lines  []DiffLine
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
