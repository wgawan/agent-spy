package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	cmd.Run()
	// Create initial commit
	testFile := filepath.Join(dir, "README.md")
	os.WriteFile(testFile, []byte("# Test\n"), 0644)
	cmd = exec.Command("git", "-C", dir, "add", ".")
	cmd.Run()
	cmd = exec.Command("git", "-C", dir, "commit", "-m", "init")
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
	cmd.Run()
	return dir
}

func TestRepoDetection(t *testing.T) {
	dir := initTestRepo(t)
	r, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	if !r.Available() {
		t.Error("expected repo to be available")
	}
}

func TestRepoDetectionNonRepo(t *testing.T) {
	dir := t.TempDir()
	r, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	if r.Available() {
		t.Error("expected repo to NOT be available for non-git dir")
	}
}

func TestBranch(t *testing.T) {
	dir := initTestRepo(t)
	r, _ := Open(dir)
	branch := r.Branch()
	if branch != "master" && branch != "main" {
		t.Errorf("expected master or main, got %q", branch)
	}
}

func TestBranchNonRepo(t *testing.T) {
	dir := t.TempDir()
	r, _ := Open(dir)
	if r.Branch() != "" {
		t.Error("expected empty branch for non-repo")
	}
}

func TestDiffFirstSeeTrackedFile(t *testing.T) {
	dir := initTestRepo(t)
	r, _ := Open(dir)

	// Modify committed file — first Diff call uses HEAD as baseline
	testFile := filepath.Join(dir, "README.md")
	os.WriteFile(testFile, []byte("# Test\n\nNew content\n"), 0644)

	diff, err := r.Diff("README.md")
	if err != nil {
		t.Fatalf("Diff() error: %v", err)
	}
	if !diff.Available {
		t.Error("expected diff to be available")
	}
	if diff.Stats.Added == 0 {
		t.Error("expected added lines > 0")
	}
}

func TestDiffNewFileAllAdditions(t *testing.T) {
	dir := initTestRepo(t)
	r, _ := Open(dir)

	// Create a new untracked file — first Diff shows all as additions
	os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main\n\nfunc hello() {}\n"), 0644)

	diff, err := r.Diff("new.go")
	if err != nil {
		t.Fatalf("Diff() error: %v", err)
	}
	if !diff.Available {
		t.Error("expected diff to be available")
	}
	if diff.Stats.Added != 3 {
		t.Errorf("expected 3 added lines, got %d", diff.Stats.Added)
	}
}

func TestDiffSnapshotBetweenEdits(t *testing.T) {
	dir := initTestRepo(t)
	r, _ := Open(dir)

	// First write — new file, all additions
	f := filepath.Join(dir, "app.go")
	os.WriteFile(f, []byte("package main\n\nfunc main() {}\n"), 0644)
	diff1, _ := r.Diff("app.go")
	if diff1.Stats.Added != 3 {
		t.Errorf("first diff: expected 3 added, got %d", diff1.Stats.Added)
	}

	// Second write — modify one line. Should show deletion + addition, not all additions.
	os.WriteFile(f, []byte("package main\n\nfunc main() { fmt.Println(\"hi\") }\n"), 0644)
	diff2, _ := r.Diff("app.go")
	if !diff2.Available {
		t.Fatal("expected second diff to be available")
	}
	if diff2.Stats.Deleted == 0 {
		t.Error("second diff: expected deleted lines > 0 (old line removed)")
	}
	if diff2.Stats.Added == 0 {
		t.Error("second diff: expected added lines > 0 (new line added)")
	}
	// Should NOT be 3 added lines — only the changed line(s)
	if diff2.Stats.Added >= 3 {
		t.Errorf("second diff: got %d added lines, expected fewer than 3 (only changed lines)", diff2.Stats.Added)
	}
}

func TestDiffDeletedLines(t *testing.T) {
	dir := initTestRepo(t)
	r, _ := Open(dir)

	// Modify committed file: replace heading with different content
	testFile := filepath.Join(dir, "README.md")
	os.WriteFile(testFile, []byte("Changed heading\n"), 0644)

	diff, err := r.Diff("README.md")
	if err != nil {
		t.Fatalf("Diff() error: %v", err)
	}
	if !diff.Available {
		t.Error("expected diff to be available")
	}
	if diff.Stats.Deleted == 0 {
		t.Error("expected deleted lines > 0")
	}
	if diff.Stats.Added == 0 {
		t.Error("expected added lines > 0")
	}
}

func TestDiffNonExistentFile(t *testing.T) {
	dir := t.TempDir()
	r, _ := Open(dir)
	diff, err := r.Diff("anything.txt")
	if err != nil {
		t.Fatalf("Diff() error: %v", err)
	}
	if diff.Available {
		t.Error("expected diff to NOT be available for non-existent file")
	}
}

func TestDiffWorksWithoutGit(t *testing.T) {
	dir := t.TempDir()
	r, _ := Open(dir)

	// Create a file in a non-git directory
	f := filepath.Join(dir, "hello.txt")
	os.WriteFile(f, []byte("hello world\n"), 0644)

	diff, _ := r.Diff("hello.txt")
	if !diff.Available {
		t.Error("expected diff to be available even without git")
	}
	if diff.Stats.Added != 1 {
		t.Errorf("expected 1 added line, got %d", diff.Stats.Added)
	}
}

func TestGitignorePatterns(t *testing.T) {
	dir := initTestRepo(t)

	// Create .gitignore
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\ntmp/\n"), 0644)

	r, _ := Open(dir)
	patterns := r.IgnorePatterns()

	if len(patterns) == 0 {
		t.Fatal("expected ignore patterns")
	}

	// Should contain *.log and tmp/
	found := map[string]bool{}
	for _, p := range patterns {
		found[p] = true
	}
	if !found["*.log"] {
		t.Error("expected *.log in patterns")
	}
	if !found["tmp/"] {
		t.Error("expected tmp/ in patterns")
	}
}

func TestGitignorePatternsNonRepo(t *testing.T) {
	dir := t.TempDir()
	r, _ := Open(dir)
	patterns := r.IgnorePatterns()
	if len(patterns) != 0 {
		t.Errorf("expected no patterns for non-repo, got %v", patterns)
	}
}
