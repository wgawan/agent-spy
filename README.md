# agent-spy

**Watch what your AI agent is *actually* doing to your codebase.**

`agent-spy` is a live terminal dashboard that monitors file changes in real time. Point it at your project directory, let your AI coding agent loose, and see every create, modify, and delete as it happens — complete with syntax-highlighted unified diffs showing exactly what changed in each edit.

Built for developers who want visibility into autonomous agents (Claude Code, Cursor, Copilot Workspace, Aider, etc.) but useful any time you want a real-time view of filesystem activity.

```
 agent-spy: ~/projects/myapp        3 files │ +14 -6 │ ▶ 2m 31s │ git:main
┌─ Events ─────────────────┐┌──────────────────────────────────────────┐
│▶ 14:23:07 M src/app.go   ││ M src/app.go 14:23:07                   │
│  14:23:05 + src/util.go   ││ @@ -12,7 +12,9 @@                       │
│  14:23:01 M go.mod        ││    func main() {                        │
│  14:22:58 + src/app.go    ││ -     log.Println("starting")           │
│                           ││ +     log.Println("starting server")    │
│                           ││ +     if err := run(); err != nil {     │
│                           ││ +         log.Fatal(err)                │
│                           ││ +     }                                 │
│                           ││    }                                    │
│                           ││                                         │
│                           ││  +4 -1                                  │
└───────────────────────────┘└──────────────────────────────────────────┘
 ↑↓:select  a:auto-scroll[off]  F:fullscreen  f:filter  c:clear  q:quit
```

## Why agent-spy?

AI coding agents work fast. They create, modify, and delete files across your project in seconds. Without visibility, you're trusting blindly. `agent-spy` gives you a live X-ray into what's happening:

- **See every file change the moment it happens** — not after the agent says "done"
- **View accurate diffs between edits** — not just "everything vs HEAD." Each event shows what actually changed in *that specific edit*, even in repos with no commits
- **Spot problems early** — catch an agent going off the rails before it rewrites half your codebase
- **Keep a log** — optionally write all events to a file for post-mortem analysis

## Quickstart

```bash
# Install
go install github.com/wally/agent-spy@latest

# Or build from source
git clone https://github.com/wgawan/agent-spy.git
cd agent-spy
make build

# Run — watch the current directory
./agent-spy

# Watch a specific project
./agent-spy ~/projects/myapp
```

## Keyboard Controls

| Key | Action |
|---|---|
| `↑` / `k` | Select previous event |
| `↓` / `j` | Select next event |
| `a` | Toggle auto-scroll (jump to newest event) |
| `F` | Toggle fullscreen diff view |
| `f` | Filter events by path |
| `c` | Clear all events |
| `Ctrl+d` | Scroll diff down |
| `Ctrl+u` | Scroll diff up |
| `q` / `Ctrl+c` | Quit |

## Features

### Live event stream
Every file create, modify, delete, and rename appears instantly in the event list with timestamps and operation indicators (`+` create, `M` modify, `D` delete, `R` rename).

### Snapshot-based diffs
Diffs show what changed in *each specific edit*, not the cumulative difference from HEAD. When an agent modifies a file three times, you see three separate diffs — each showing only what that edit changed. For tracked files seen for the first time, the diff uses the git HEAD version as a baseline.

### Smart noise filtering
Editor temp files, build artifacts, lock files, and other noise are automatically filtered out:

- **Directories:** `.git`, `node_modules`, `vendor`, `.venv`, `__pycache__`, `build`, `dist`, `.next`, `.nuxt`, `target`
- **Files:** `.DS_Store`, `Thumbs.db`, `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`
- **Extensions:** `.lock`, `.pyc`, `.o`, `.class`, `.swp`, `.swo`, `.swn`
- **Editor temps:** vim swap files, backup files (`~` suffix), vim test files (`4913`)

Patterns from your `.gitignore` are also respected automatically.

### Event debouncing
Rapid-fire filesystem events (common when editors save files) are debounced into single events. The debounce window is configurable. Debounced events show a count indicator like `(x3)`.

### Git integration
When run inside a git repository, `agent-spy` displays the current branch in the stats bar and respects `.gitignore` patterns. Git integration can be disabled with `--no-git`.

### Event logging
Write all events to a file for later analysis with `--log events.log`.

## CLI Flags

```
Usage: agent-spy [flags] [path]

Flags:
  -debounce int    debounce interval in milliseconds (default 500)
  -filter string   additional exclude patterns (can be specified multiple times)
  -log string      write events to log file
  -no-git          disable git integration
  -version         print version
```

### Examples

```bash
# Watch current directory with defaults
agent-spy

# Watch a specific project with a shorter debounce
agent-spy -debounce 200 ~/projects/myapp

# Exclude additional patterns
agent-spy -filter "*.tmp" -filter "logs/"

# Log events to a file while watching
agent-spy -log session.log ~/projects/myapp

# Watch a non-git directory (skip git detection)
agent-spy -no-git /tmp/scratch
```

## Requirements

- Go 1.19+
- `git` CLI (for diff functionality; the tool works without it but diffs will be unavailable)

## Building from source

```bash
git clone https://github.com/wgawan/agent-spy.git
cd agent-spy
make build    # produces ./agent-spy binary
make test     # run tests
make install  # install to $GOPATH/bin
make clean    # remove binary
```

## Architecture

```
main.go                  CLI flags, wiring
internal/
  watcher/               fsnotify-based recursive watcher + smart filtering + debouncing
  git/                   git repo detection, branch info, snapshot-based diffing
  tui/                   bubbletea TUI (model, layout, event list, detail pane, styles)
  types/                 shared types (FileEvent, DiffResult, Operation)
  logger/                structured event logging
```

## License

MIT
