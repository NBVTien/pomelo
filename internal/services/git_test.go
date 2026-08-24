package services

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s (%v)", args, out, err)
	}
}

func TestCreateWorktreeRecoversStaleRegistration(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "myrepo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "init", "-q")
	gitRun(t, repo, "branch", "-M", "main")
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-q", "-m", "init")

	stale := filepath.Join(root, "stale")
	gitRun(t, repo, "worktree", "add", "-q", "-b", "feat", stale, "main")
	if err := os.RemoveAll(stale); err != nil {
		t.Fatal(err)
	}

	ws := filepath.Join(root, "workspace--x")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := CreateWorktreeFromBase(repo, "feat", "main", nil, ws)
	if err != nil {
		t.Fatalf("expected auto-recovery, got: %v", err)
	}
	if st, e := os.Stat(got); e != nil || !st.IsDir() {
		t.Fatalf("worktree dir not created: %s", got)
	}
}
