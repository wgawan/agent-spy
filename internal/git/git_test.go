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
