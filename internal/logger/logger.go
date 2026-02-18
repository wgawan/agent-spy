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
