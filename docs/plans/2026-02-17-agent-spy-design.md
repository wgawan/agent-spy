# agent-spy Design

A live TUI for watching what coding agents (or any editor) change in your project.

## Usage

```
agent-spy [path]           # Watch current dir or specified path
agent-spy ./my-project     # Watch a specific project

Flags:
  --debounce <ms>          # Debounce interval (default: 500ms)
  --filter <glob>          # Additional exclude patterns
  --no-git                 # Disable git integration even in git repos
  --log <file>             # Also write events to a log file
```

## Tech Stack

- **Language:** Go
- **TUI:** bubbletea + lipgloss + bubbles (Charm ecosystem)
- **File watching:** fsnotify
- **Git integration:** go-git (optional, pure Go)

## Architecture

```
main.go
  Parse flags -> Init watcher -> Init TUI -> Run

Packages:
  watcher/   - fsnotify, debouncing, filtering
  tui/       - bubbletea application, 3-zone layout
  git/       - go-git integration, diff, gitignore
```

### Data Flow

1. `watcher` uses fsnotify to watch the target directory recursively
2. Raw filesystem events are debounced (configurable, default 500ms) and filtered
3. `FileEvent` structs are sent on a channel to the TUI
4. TUI renders events in the event list; calls `git` package on demand for diffs

### Key Types

```go
type FileEvent struct {
    Path      string
    Op        Operation  // Create, Modify, Delete, Rename
    Timestamp time.Time
    SubEvents []FileEvent // Individual changes before debounce
}

type DiffResult struct {
    Available bool
    Hunks     []DiffHunk
    Stats     DiffStats  // lines added/removed
}
```

## TUI Layout

```
+-- agent-spy: ~/projects/my-app ---------------------------+
| 12 files | +847 -203 | > 3m 22s | git:main             (1)|
+-- Events (up/down) ------+-- Detail ----------------------+
| 14:03:02 M src/app.ts   | --- a/src/app.ts              |(2)
| 14:03:01 + lib/util.ts  | +++ b/src/app.ts              |
|>14:02:58 M README.md(x3)| @@ -12,3 +12,7 @@             |(3)
| 14:02:55 D old.txt      |  existing line                 |
|                          | +new line added                |
|                          | -old line removed              |
+----------------------------+------------------------------+
| up/dn:select  enter:expand  F:fullscreen  f:filter q:quit |
+------------------------------------------------------------+
```

### Zones

1. **Stats bar** - File count, total +/- lines, elapsed time, git branch
2. **Event list** - Scrollable, shows timestamp + operation + path. Debounced events show (xN)
3. **Detail pane** - Diff for selected event, color-coded (green adds, red deletes)

### Keybindings

| Key | Action |
|-----|--------|
| `up/down` or `j/k` | Navigate event list |
| `enter` | Expand debounced event to sub-events |
| `F` | Toggle detail pane fullscreen |
| `Esc` | Exit fullscreen |
| `f` | Open filter input |
| `c` | Clear event history |
| `q` / `Ctrl+C` | Quit |

### Fullscreen Mode

Detail pane takes over the entire area below the stats bar. `F` or `Esc` to return.

### Debouncing

Multiple writes to the same file within the debounce window (default 500ms) are collapsed into a single event with a `(xN)` indicator. The individual sub-events are preserved and can be viewed by pressing `enter` on the debounced event.

## Git Integration (Optional)

When the watched directory is a git repo:
- Diffs are generated via go-git (pure Go, no git binary needed)
- .gitignore patterns are respected for filtering
- Stats bar shows current branch
- +/- line counts shown per event and in aggregate

When not in a git repo:
- Detail pane shows file content for creates
- "file deleted" for deletes
- "no diff available (not a git repo)" for modifications
- Everything else works normally

## Smart Default Filters

Always excluded:
- `.git/`

Excluded by default (overridable):
- `node_modules/`, `vendor/`, `.venv/`, `__pycache__/`
- `*.lock`, `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`
- `.DS_Store`, `Thumbs.db`
- `build/`, `dist/`, `.next/`, `.nuxt/`, `target/`
- `*.o`, `*.pyc`, `*.class`

Plus `.gitignore` patterns when in a git repo.

## Log File (--log)

When specified, events are appended in structured format:

```
2026-02-17T14:03:02Z MODIFY src/app.ts +12 -3
2026-02-17T14:03:01Z CREATE lib/util.ts +28
2026-02-17T14:02:55Z DELETE old.txt -44
```
