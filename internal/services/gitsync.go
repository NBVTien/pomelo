package services

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func gitOut(dir string, env []string, timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	if env != nil {
		cmd.Env = env
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return out.Bytes(), err
}

func GitBranch(wt string) string {
	out, err := gitOut(wt, nil, 10*time.Second, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func PushWorktree(wt string) {
	head := GitBranch(wt)
	if head == "" || head == "HEAD" {
		return
	}
	_, _ = RunTimeout(30*time.Second, wt, "git", "push", "origin", "HEAD:refs/heads/"+head)
	ref, changed, err := WriteWIPSnapshot(wt, head)
	if err != nil || !changed {
		return
	}
	_, _ = RunTimeout(30*time.Second, wt, "git", "push", "--force", "origin", ref+":"+ref)
}

func WriteWIPSnapshot(wt, head string) (ref string, changed bool, err error) {
	ref = "refs/pom-wip/" + head
	status, err := gitOut(wt, nil, 15*time.Second, "status", "--porcelain")
	if err != nil {
		return ref, false, fmt.Errorf("status: %w", err)
	}
	if len(bytes.TrimSpace(status)) == 0 {
		return ref, false, nil
	}
	idxDir, err := os.MkdirTemp("", "pom-wip-idx")
	if err != nil {
		return ref, false, err
	}
	defer os.RemoveAll(idxDir)
	env := append(os.Environ(),
		"GIT_INDEX_FILE="+filepath.Join(idxDir, "index"),
		"GIT_AUTHOR_NAME=pom", "GIT_AUTHOR_EMAIL=pom@localhost",
		"GIT_COMMITTER_NAME=pom", "GIT_COMMITTER_EMAIL=pom@localhost",
	)
	if out, e := gitOut(wt, env, 15*time.Second, "read-tree", "HEAD"); e != nil {
		return ref, false, fmt.Errorf("read-tree: %w (%s)", e, out)
	}
	if out, e := gitOut(wt, env, 30*time.Second, "add", "-A"); e != nil {
		return ref, false, fmt.Errorf("add: %w (%s)", e, out)
	}
	treeOut, e := gitOut(wt, env, 15*time.Second, "write-tree")
	if e != nil {
		return ref, false, fmt.Errorf("write-tree: %w (%s)", e, treeOut)
	}
	tree := strings.TrimSpace(string(treeOut))
	if prev, e := gitOut(wt, nil, 10*time.Second, "rev-parse", "-q", "--verify", ref+"^{tree}"); e == nil {
		if strings.TrimSpace(string(prev)) == tree {
			return ref, false, nil
		}
	}
	commitOut, e := gitOut(wt, env, 15*time.Second, "commit-tree", tree, "-p", "HEAD", "-m", "pom-wip snapshot")
	if e != nil {
		return ref, false, fmt.Errorf("commit-tree: %w (%s)", e, commitOut)
	}
	if out, e := gitOut(wt, nil, 10*time.Second, "update-ref", ref, strings.TrimSpace(string(commitOut))); e != nil {
		return ref, false, fmt.Errorf("update-ref: %w (%s)", e, out)
	}
	return ref, true, nil
}

func PruneWIPRefs(wt string) {
	local := map[string]bool{}
	if out, err := gitOut(wt, nil, 15*time.Second, "worktree", "list", "--porcelain"); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if b := strings.TrimPrefix(line, "branch refs/heads/"); b != line {
				local[b] = true
			}
		}
	}
	out, err := gitOut(wt, nil, 30*time.Second, "ls-remote", "origin", "refs/pom-wip/*")
	if err != nil {
		return
	}
	const pfx = "refs/pom-wip/"
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		i := strings.Index(line, pfx)
		if i < 0 {
			continue
		}
		if br := line[i+len(pfx):]; br != "" && !local[br] {
			_, _ = RunTimeout(20*time.Second, wt, "git", "push", "origin", "--delete", pfx+br)
		}
	}
}

func TakeoverWorktree(wt string) string {
	head := GitBranch(wt)
	if head == "" || head == "HEAD" {
		return "skipped (detached)"
	}
	if _, err := RunTimeout(60*time.Second, wt, "git", "fetch", "origin"); err != nil {
		return "fetch failed (offline?)"
	}
	dirty := false
	if st, _ := gitOut(wt, nil, 15*time.Second, "status", "--porcelain"); len(bytes.TrimSpace(st)) > 0 {
		dirty = true
	}
	msgs := []string{}
	if _, err := gitOut(wt, nil, 30*time.Second, "merge", "--ff-only", "origin/"+head); err == nil {
		msgs = append(msgs, "fast-forwarded")
	} else {
		msgs = append(msgs, "no ff (up-to-date or diverged)")
	}
	ref := "refs/pom-wip/" + head
	if _, err := gitOut(wt, nil, 30*time.Second, "fetch", "origin", ref+":"+ref); err == nil {
		if dirty {
			msgs = append(msgs, "WIP available but tree dirty — skipped")
		} else if out, err := gitOut(wt, nil, 30*time.Second, "checkout", ref, "--", "."); err == nil {
			msgs = append(msgs, "restored WIP")
		} else {
			msgs = append(msgs, fmt.Sprintf("WIP apply failed: %s", strings.TrimSpace(string(out))))
		}
	}
	return strings.Join(msgs, "; ")
}
