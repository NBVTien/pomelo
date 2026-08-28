package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pomelohq/pomelo/internal/services"
)

// ReviewGet returns the authored review artifact for a workspace, or {"exists":false}
// when none has been written yet. Reviews are workspace-scoped (a workspace spans
// several repos), so anchors inside are repo-qualified. Stored at the project root
// under .pom/reviews/<branch>.json.
func (s *Server) ReviewGet(branch string, isMain bool) []byte {
	empty := []byte(`{"exists":false}`)
	root := s.WorkspaceRoot
	if root == "" {
		return empty
	}
	name := services.BranchSafe(branch)
	if name == "" {
		return empty
	}
	b, err := os.ReadFile(filepath.Join(root, ".pom", "reviews", name+".json"))
	if err != nil {
		return empty
	}
	return b
}

// FilePeek returns the contents of repo/path so a code anchor can show a peek. It
// reads the pushed head ref first (so it matches the PR even when the local worktree
// is behind), then HEAD, then the working tree. Path is confined to the worktree.
func (s *Server) FilePeek(branch, repo, path string, isMain bool) []byte {
	if strings.Contains(path, "..") || filepath.IsAbs(path) {
		return []byte(`{"error":"bad path"}`)
	}
	repo = s.resolveRepoDir(repo)
	wt := repoWorktreePath(s.WorkspaceRoot, repo, branch, isMain)
	var content string
	refs := []string{}
	if head := services.CurrentBranch(wt); head != "" {
		refs = append(refs, "origin/"+head)
	}
	refs = append(refs, "HEAD")
	for _, ref := range refs {
		if out, err := services.RunTimeout(6*time.Second, wt, "git", "show", ref+":"+path); err == nil {
			content = string(out)
			break
		}
	}
	if content == "" { // last resort: the working tree
		if b, err := os.ReadFile(filepath.Join(wt, path)); err == nil {
			content = string(b)
		} else {
			return []byte(`{"error":"not found"}`)
		}
	}
	out, _ := json.Marshal(map[string]any{"repo": repo, "path": path, "content": content})
	return out
}

// resolveRepoDir maps a repo reference that may be a pom.yml alias to the actual
// worktree directory name, so review anchors authored with friendly aliases still
// resolve. Returns the input unchanged when no match.
func (s *Server) resolveRepoDir(repo string) string {
	cfg := s.cfg()
	if cfg == nil || repo == "" {
		return repo
	}
	if _, ok := cfg.Repos[repo]; ok {
		return repo
	}
	for dir, d := range cfg.Repos {
		if d != nil && d.Alias == repo {
			return dir
		}
	}
	return repo
}
