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
// initRemote builds a bare origin plus one clone holding a single commit on
// `branch`. Every branch name is passed explicitly: CI has no
// init.defaultBranch, so anything inherited from config differs from a dev box.
func initRemote(t *testing.T, root, branch string) (remote, clone string) {
	t.Helper()
	remote = filepath.Join(root, "remote.git")
	gitRun(t, root, "init", "-q", "--bare", "--initial-branch="+branch, "remote.git")

	clone = filepath.Join(root, "mine")
	gitRun(t, root, "clone", "-q", remote, "mine")
	gitRun(t, clone, "checkout", "-q", "-B", branch)
	writeFile(t, clone, "f.txt", "line1\n")
	gitRun(t, clone, "add", ".")
	gitRun(t, clone, "commit", "-q", "-m", "init")
	gitRun(t, clone, "push", "-q", "-u", "origin", branch)
	return remote, clone
}

// teammatePush clones the remote fresh and pushes one commit to branch.
func teammatePush(t *testing.T, root, remote, branch, name, content string) {
	t.Helper()
	dir := filepath.Join(root, "theirs")
	gitRun(t, root, "clone", "-q", "--branch", branch, remote, "theirs")
	writeFile(t, dir, name, content)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-q", "-m", "teammate work")
	gitRun(t, dir, "push", "-q", "origin", branch)
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A teammate pushing to the same branch must not show up as local work.
func TestUnpushedBaseIgnoresUpstreamOnlyCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	remote, mine := initRemote(t, root, "main")

	teammatePush(t, root, remote, "main", "f.txt", "line1\ntheirs\n")

	writeFile(t, mine, "f.txt", "line1\nmine\n")
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
	_, wt := initRemote(t, root, "main")

	writeFile(t, wt, "f.txt", "line1\nuncommitted\n")

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
	remote, mine := initRemote(t, root, "main")

	if n := UpstreamBehind(mine); n != 0 {
		t.Fatalf("in sync should be 0 behind, got %d", n)
	}

	teammatePush(t, root, remote, "main", "f.txt", "line1\ntheirs\n")
	gitRun(t, mine, "fetch", "-q", "origin")

	if n := UpstreamBehind(mine); n != 1 {
		t.Fatalf("want 1 behind after teammate push, got %d", n)
	}
}

// The local stat is measured against a merge-base, so a teammate's push cannot
// change it — with or without a fetch. Only UpstreamBehind needs the remote.
func TestLocalChangeStatIsFetchIndependent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	remote, mine := initRemote(t, root, "main")

	writeFile(t, mine, "f.txt", "line1\nmine\n")
	gitRun(t, mine, "commit", "-q", "-am", "my work")

	before := UnpushedBase("main", mine)
	_, insBefore, _ := LocalChangeStat(before, mine)

	teammatePush(t, root, remote, "main", "g.txt", "theirs\n")
	gitRun(t, mine, "fetch", "-q", "origin")

	after := UnpushedBase("main", mine)
	_, insAfter, _ := LocalChangeStat(after, mine)
	if insBefore != insAfter {
		t.Fatalf("a teammate push changed the local count: %d -> %d", insBefore, insAfter)
	}
}
