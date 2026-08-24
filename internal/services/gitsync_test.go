package services

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestWriteWIPSnapshot(t *testing.T) {
	wt := t.TempDir()
	gitT(t, wt, "init", "-q", "-b", "feat-x")
	gitT(t, wt, "config", "user.email", "t@t")
	gitT(t, wt, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(wt, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, wt, "add", "-A")
	gitT(t, wt, "commit", "-qm", "init")

	if _, changed, err := WriteWIPSnapshot(wt, "feat-x"); err != nil || changed {
		t.Fatalf("clean: changed=%v err=%v", changed, err)
	}

	if err := os.WriteFile(filepath.Join(wt, "a.txt"), []byte("two-changed-body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	headBefore := gitT(t, wt, "rev-parse", "HEAD")
	ref, changed, err := WriteWIPSnapshot(wt, "feat-x")
	if err != nil || !changed {
		t.Fatalf("dirty: changed=%v err=%v", changed, err)
	}
	if gitT(t, wt, "rev-parse", "HEAD") != headBefore {
		t.Fatal("HEAD moved — snapshot must not touch HEAD")
	}
	if got := gitT(t, wt, "show", ref+":a.txt"); got != "two-changed-body" {
		t.Fatalf("snapshot content = %q, want %q", got, "two-changed-body")
	}
	if st := gitT(t, wt, "status", "--porcelain"); !strings.Contains(st, "a.txt") {
		t.Fatalf("working tree state changed unexpectedly: %q", st)
	}

	if _, changed, err := WriteWIPSnapshot(wt, "feat-x"); err != nil || changed {
		t.Fatalf("unchanged rerun: changed=%v err=%v", changed, err)
	}
}

func TestPruneWIPRefs(t *testing.T) {
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	gitT(t, base, "init", "-q", "--bare", origin)

	wt := filepath.Join(base, "clone")
	gitT(t, base, "clone", "-q", origin, wt)
	gitT(t, wt, "config", "user.email", "t@t")
	gitT(t, wt, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(wt, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, wt, "add", "-A")
	gitT(t, wt, "commit", "-qm", "c")
	gitT(t, wt, "push", "-q", "origin", "HEAD")
	head := gitT(t, wt, "rev-parse", "HEAD")
	branch := gitT(t, wt, "rev-parse", "--abbrev-ref", "HEAD")

	gitT(t, wt, "update-ref", "refs/pom-wip/"+branch, head)
	gitT(t, wt, "update-ref", "refs/pom-wip/gone-branch", head)
	gitT(t, wt, "push", "-q", "origin", "refs/pom-wip/"+branch, "refs/pom-wip/gone-branch")

	PruneWIPRefs(wt)

	remote := gitT(t, wt, "ls-remote", "origin", "refs/pom-wip/*")
	if !strings.Contains(remote, "refs/pom-wip/"+branch) {
		t.Fatalf("live branch wip ref was wrongly pruned:\n%s", remote)
	}
	if strings.Contains(remote, "gone-branch") {
		t.Fatalf("stale wip ref not pruned:\n%s", remote)
	}
}

func TestGitBranch(t *testing.T) {
	wt := t.TempDir()
	gitT(t, wt, "init", "-q", "-b", "my-branch")
	gitT(t, wt, "config", "user.email", "t@t")
	gitT(t, wt, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(wt, "x"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, wt, "add", "-A")
	gitT(t, wt, "commit", "-qm", "c")
	if b := GitBranch(wt); b != "my-branch" {
		t.Fatalf("GitBranch = %q, want my-branch", b)
	}
}
