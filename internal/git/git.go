package git

import (
	"bufio"
	"fmt"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
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

	wt, err := r.repo.Worktree()
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}

	status, err := wt.Status()
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}

	fileStatus := status.File(relPath)
	if fileStatus.Worktree == gogit.Unmodified && fileStatus.Staging == gogit.Unmodified {
		return types.DiffResult{Available: false, Error: "file not modified"}, nil
	}

	// Get HEAD tree
	ref, err := r.repo.Head()
	if err != nil {
		// No commits yet - entire file is "added"
		return r.diffNewFile(relPath)
	}

	commit, err := r.repo.CommitObject(ref.Hash())
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}

	tree, err := commit.Tree()
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}

	// Try to get the file from HEAD
	_, err = tree.File(relPath)
	if err != nil {
		// File doesn't exist in HEAD - it's new
		return r.diffNewFile(relPath)
	}

	// Get patch between HEAD and worktree
	return r.getWorkingDirPatch(tree, relPath)
}

func (r *Repo) diffNewFile(relPath string) (types.DiffResult, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}

	fs := wt.Filesystem
	f, err := fs.Open(relPath)
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}
	defer f.Close()

	buf := make([]byte, 32*1024)
	n, _ := f.Read(buf)
	content := string(buf[:n])
	lines := strings.Split(content, "\n")

	var diffLines []types.DiffLine
	for i, line := range lines {
		// Skip the trailing empty string from Split
		if i == len(lines)-1 && line == "" {
			continue
		}
		diffLines = append(diffLines, types.DiffLine{
			Content: line,
			Type:    types.DiffLineAdd,
		})
	}

	return types.DiffResult{
		Available: true,
		Hunks: []types.DiffHunk{
			{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(diffLines)), Lines: diffLines},
		},
		Stats: types.DiffStats{Added: len(diffLines)},
	}, nil
}

func (r *Repo) getWorkingDirPatch(headTree *object.Tree, relPath string) (types.DiffResult, error) {
	// Read current file from working directory
	wt, _ := r.repo.Worktree()
	fs := wt.Filesystem
	f, err := fs.Open(relPath)
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}
	defer f.Close()

	buf := make([]byte, 64*1024)
	n, _ := f.Read(buf)
	newContent := string(buf[:n])

	// Read old file from HEAD
	headFile, err := headTree.File(relPath)
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}
	oldContent, err := headFile.Contents()
	if err != nil {
		return types.DiffResult{Available: false, Error: err.Error()}, nil
	}

	return computeSimpleDiff(oldContent, newContent), nil
}

func computeSimpleDiff(old, new string) types.DiffResult {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	var lines []types.DiffLine
	added, deleted := 0, 0

	oldSet := make(map[string]bool)
	for _, l := range oldLines {
		oldSet[l] = true
	}
	newSet := make(map[string]bool)
	for _, l := range newLines {
		newSet[l] = true
	}

	for _, l := range oldLines {
		if !newSet[l] && l != "" {
			lines = append(lines, types.DiffLine{Content: l, Type: types.DiffLineDelete})
			deleted++
		} else if l != "" {
			lines = append(lines, types.DiffLine{Content: l, Type: types.DiffLineContext})
		}
	}
	for _, l := range newLines {
		if !oldSet[l] && l != "" {
			lines = append(lines, types.DiffLine{Content: l, Type: types.DiffLineAdd})
			added++
		}
	}

	var hunks []types.DiffHunk
	if len(lines) > 0 {
		hunks = append(hunks, types.DiffHunk{
			Header: fmt.Sprintf("@@ -%d +%d @@", len(oldLines), len(newLines)),
			Lines:  lines,
		})
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
