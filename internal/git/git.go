package git

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/wally/agent-spy/internal/types"
)

type Repo struct {
	repo      *gogit.Repository
	path      string
	snapshots map[string]string // file path -> content at last event
}

func Open(path string) (*Repo, error) {
	r := &Repo{path: path, snapshots: make(map[string]string)}
	repo, err := gogit.PlainOpen(path)
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

func (r *Repo) Diff(relPath string) (types.DiffResult, error) {
	// Read current file content
	absPath := filepath.Join(r.path, relPath)
	currentBytes, err := os.ReadFile(absPath)
	if err != nil {
		// File was deleted
		prev, hasPrev := r.snapshots[relPath]
		delete(r.snapshots, relPath)
		if hasPrev && prev != "" {
			return r.diffStrings(prev, "", relPath)
		}
		return types.DiffResult{Available: false, Error: "file not readable"}, nil
	}
	current := string(currentBytes)

	// Get the baseline to diff against
	prev, hasPrev := r.snapshots[relPath]

	// Update snapshot for next time
	r.snapshots[relPath] = current

	if !hasPrev {
		// First time seeing this file — try git HEAD as baseline
		headContent := r.getHeadContent(relPath)
		if headContent != "" && headContent != current {
			return r.diffStrings(headContent, current, relPath)
		}
		if headContent == current {
			return types.DiffResult{Available: false, Error: "no changes"}, nil
		}
		// No HEAD content (untracked/new repo) — show all as additions
		return r.diffStrings("", current, relPath)
	}

	if prev == current {
		return types.DiffResult{Available: false, Error: "no changes"}, nil
	}

	return r.diffStrings(prev, current, relPath)
}

// getHeadContent returns the file content from HEAD, or "" if unavailable.
func (r *Repo) getHeadContent(relPath string) string {
	if r.repo == nil {
		return ""
	}
	cmd := exec.Command("git", "-C", r.path, "show", "HEAD:"+relPath)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// diffStrings produces a unified diff between old and new content using git diff --no-index.
func (r *Repo) diffStrings(oldContent, newContent, relPath string) (types.DiffResult, error) {
	tmpDir, err := os.MkdirTemp("", "agent-spy-diff-*")
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}
	defer os.RemoveAll(tmpDir)

	oldFile := filepath.Join(tmpDir, "old")
	newFile := filepath.Join(tmpDir, "new")

	if err := os.WriteFile(oldFile, []byte(oldContent), 0644); err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}
	if err := os.WriteFile(newFile, []byte(newContent), 0644); err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}

	cmd := exec.Command("git", "diff", "--no-index", "--", oldFile, newFile)
	out, _ := cmd.Output() // exit code 1 expected when files differ

	output := string(out)
	if output == "" {
		return types.DiffResult{Available: false, Error: "no changes"}, nil
	}

	return parseDiffOutput(output), nil
}

// parseDiffOutput parses unified diff output into DiffResult.
func parseDiffOutput(output string) types.DiffResult {
	lines := strings.Split(output, "\n")
	var hunks []types.DiffHunk
	var currentHunk *types.DiffHunk
	added, deleted := 0, 0

	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			// New hunk header
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
			}
			currentHunk = &types.DiffHunk{Header: line}
			continue
		}

		if currentHunk == nil {
			// Skip diff header lines (diff --git, index, ---, +++)
			continue
		}

		if strings.HasPrefix(line, "+") {
			currentHunk.Lines = append(currentHunk.Lines, types.DiffLine{
				Content: line[1:],
				Type:    types.DiffLineAdd,
			})
			added++
		} else if strings.HasPrefix(line, "-") {
			currentHunk.Lines = append(currentHunk.Lines, types.DiffLine{
				Content: line[1:],
				Type:    types.DiffLineDelete,
			})
			deleted++
		} else if strings.HasPrefix(line, " ") {
			currentHunk.Lines = append(currentHunk.Lines, types.DiffLine{
				Content: line[1:],
				Type:    types.DiffLineContext,
			})
		}
	}

	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}

	if len(hunks) == 0 {
		return types.DiffResult{Available: false, Error: "no changes"}
	}

	return types.DiffResult{
		Available: true,
		Hunks:     hunks,
		Stats:     types.DiffStats{Added: added, Deleted: deleted},
	}
}

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

// StatusSummary returns a short string like "3 modified, 1 new" for the stats bar.
func (r *Repo) StatusSummary() string {
	if r.repo == nil {
		return ""
	}
	cmd := exec.Command("git", "-C", r.path, "diff", "--stat", "--shortstat", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "clean"
	}
	return fmt.Sprintf("%s", s)
}
