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

// A teammate pushing to the same branch must not show up as local work.
func TestUnpushedBaseIgnoresUpstreamOnlyCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	gitRun(t, root, "init", "-q", "--bare", "remote.git")

	mine := filepath.Join(root, "mine")
	gitRun(t, root, "clone", "-q", remote, "mine")
	if err := os.WriteFile(filepath.Join(mine, "f.txt"), []byte("line1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, mine, "add", ".")
	gitRun(t, mine, "commit", "-q", "-m", "init")
	gitRun(t, mine, "branch", "-M", "main")
	gitRun(t, mine, "push", "-q", "-u", "origin", "main")

	theirs := filepath.Join(root, "theirs")
	gitRun(t, root, "clone", "-q", remote, "theirs")
	if err := os.WriteFile(filepath.Join(theirs, "f.txt"), []byte("line1\ntheirs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, theirs, "commit", "-q", "-am", "teammate work")
	gitRun(t, theirs, "push", "-q", "origin", "main")

	if err := os.WriteFile(filepath.Join(mine, "f.txt"), []byte("line1\nmine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, mine, "commit", "-q", "-am", "my work")
	gitRun(t, mine, "fetch", "-q", "origin")

	base := UnpushedBase("main", mine)
	files, ins, del := LocalChangeStat(base, mine)
	if files != 1 || ins != 1 || del != 0 {
		t.Fatalf("want only my own line (1 file, 1 insertion, 0 deletions), got %d/%d/%d", files, ins, del)
	}
}

// Uncommitted work still counts as a local change.
func TestUnpushedBaseIncludesWorktreeChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	gitRun(t, root, "init", "-q", "--bare", "remote.git")

	wt := filepath.Join(root, "wt")
	gitRun(t, root, "clone", "-q", remote, "wt")
	if err := os.WriteFile(filepath.Join(wt, "f.txt"), []byte("line1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wt, "add", ".")
	gitRun(t, wt, "commit", "-q", "-m", "init")
	gitRun(t, wt, "branch", "-M", "main")
	gitRun(t, wt, "push", "-q", "-u", "origin", "main")

	if err := os.WriteFile(filepath.Join(wt, "f.txt"), []byte("line1\nuncommitted\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := UnpushedBase("main", wt)
	files, ins, _ := LocalChangeStat(base, wt)
	if files != 1 || ins != 1 {
		t.Fatalf("uncommitted change should count, got %d files / %d insertions", files, ins)
	}
}

func TestUpstreamBehindCountsTeammateCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	gitRun(t, root, "init", "-q", "--bare", "remote.git")

	mine := filepath.Join(root, "mine")
	gitRun(t, root, "clone", "-q", remote, "mine")
	if err := os.WriteFile(filepath.Join(mine, "f.txt"), []byte("line1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, mine, "add", ".")
	gitRun(t, mine, "commit", "-q", "-m", "init")
	gitRun(t, mine, "branch", "-M", "main")
	gitRun(t, mine, "push", "-q", "-u", "origin", "main")

	if n := UpstreamBehind(mine); n != 0 {
		t.Fatalf("in sync should be 0 behind, got %d", n)
	}

	theirs := filepath.Join(root, "theirs")
	gitRun(t, root, "clone", "-q", remote, "theirs")
	if err := os.WriteFile(filepath.Join(theirs, "f.txt"), []byte("line1\ntheirs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, theirs, "commit", "-q", "-am", "teammate work")
	gitRun(t, theirs, "push", "-q", "origin", "main")

	gitRun(t, mine, "fetch", "-q", "origin")
	if n := UpstreamBehind(mine); n != 1 {
		t.Fatalf("want 1 behind after teammate push, got %d", n)
	}
}
