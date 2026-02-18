package git

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/wally/agent-spy/internal/types"
)

type Repo struct {
	repo *gogit.Repository
	path string
}

func Open(path string) (*Repo, error) {
	r := &Repo{path: path}
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
	if r.repo == nil {
		return types.DiffResult{Available: false, Error: "not a git repository"}, nil
	}

	// Use git diff to get proper unified diff output.
	// Try tracked file diff first, then fall back to showing new file content.
	output, err := r.gitDiff(relPath)
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}

	if output == "" {
		// No diff from git - file might be untracked (new file)
		return r.gitDiffNewFile(relPath)
	}

	return parseDiffOutput(output), nil
}

// gitDiff runs git diff for a specific file, including both staged and unstaged changes.
func (r *Repo) gitDiff(relPath string) (string, error) {
	// git diff HEAD -- <file> shows combined staged + unstaged changes vs HEAD
	cmd := exec.Command("git", "-C", r.path, "diff", "HEAD", "--", relPath)
	out, err := cmd.Output()
	if err != nil {
		// HEAD might not exist (no commits yet), try diff against empty tree
		cmd = exec.Command("git", "-C", r.path, "diff", "--", relPath)
		out, err = cmd.Output()
		if err != nil {
			return "", err
		}
	}
	return string(out), nil
}

// gitDiffNewFile handles untracked files by showing their full content as additions.
func (r *Repo) gitDiffNewFile(relPath string) (types.DiffResult, error) {
	// Use git diff --no-index to diff /dev/null against the file
	cmd := exec.Command("git", "-C", r.path, "diff", "--no-index", "--", "/dev/null", relPath)
	out, _ := cmd.Output() // exit code 1 is expected (files differ)

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
