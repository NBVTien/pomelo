package forge

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/pomelohq/pomelo/internal/config"
	"github.com/pomelohq/pomelo/internal/plugin"
	"github.com/pomelohq/pomelo/internal/secrets"
	"github.com/pomelohq/pomelo/internal/services"
	"github.com/pomelohq/pomelo/internal/workspace"
)

type Feature struct {
	getCfg        func() *config.Config
	WorkspaceRoot string
	DefaultBranch string
}

func New(getCfg func() *config.Config, workspaceRoot, defaultBranch string) *Feature {
	tokenResolver = func() string {
		if c := getCfg(); c != nil {
			if v, ok := secrets.Get(c.Session, githubTokenSecret); ok {
				return v
			}
		}
		return ""
	}
	return &Feature{getCfg: getCfg, WorkspaceRoot: workspaceRoot, DefaultBranch: defaultBranch}
}

func (s *Feature) cfg() *config.Config { return s.getCfg() }

func (*Feature) Name() string { return "pr" }

func (s *Feature) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/api/repo/pr", s.handlePRStatus)
	mux.HandleFunc("/api/repo/commits", s.handleRepoCommits)
	mux.HandleFunc("/api/workspace/prs", s.handleWorkspacePRs)
	mux.HandleFunc("/api/prs/all", s.handleAllPRs)
	mux.HandleFunc("/api/repo/changed-files", s.handleChangedFiles)
	mux.HandleFunc("/api/repo/file", s.handleFileContent)
	mux.HandleFunc("/api/repo/pr/detail", s.handlePRDetail)
	mux.HandleFunc("/api/repo/pr/comments", s.handlePRReviewComments)
	mux.HandleFunc("/api/repo/diff", s.handleDiff)
	mux.HandleFunc("/api/workspace/local-changes", s.handleLocalChanges)
	mux.HandleFunc("/api/repo/local-diff", s.handleLocalDiff)
}

var _ plugin.HTTPProvider = (*Feature)(nil)

func defBranch(cfg *config.Config, repo string) string {
	if cfg == nil {
		return ""
	}
	return cfg.DefaultBranchFor(repo)
}

func scanSkeletons(root, defaultBranch string, known map[string]bool, _ bool) []workspace.WS {
	return workspace.Scan(root, defaultBranch, known)
}

func fetchDefault(defaultBranch, wt string) {
	base := defaultBranch
	if base == "" {
		base = "main"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "git", "-C", wt, "fetch", "--quiet", "origin", base).Run()
}

func (s *Feature) WarmLoop() {
	if s.cfg() == nil || s.WorkspaceRoot == "" {
		return
	}
	warm := func() {
		known := make(map[string]bool, len(s.cfg().Repos))
		for n := range s.cfg().Repos {
			known[n] = true
		}
		var pairs []prPair
		fetched := map[string]bool{}
		for _, ws := range workspace.Scan(s.WorkspaceRoot, s.DefaultBranch, known) {
			for _, repo := range ws.Repos {
				wt := services.RepoWorktreePath(s.WorkspaceRoot, repo.Name, ws.Branch, ws.IsMain)
				if st, err := os.Stat(wt); err != nil || !st.IsDir() {
					continue
				}
				if !fetched[repo.Name] {
					fetched[repo.Name] = true
					fetchDefault(defBranch(s.cfg(), repo.Name), wt)
				}
				if p, ok := prPairFor(repo.Name, wt); ok {
					pairs = append(pairs, p)
				}
			}
		}
		warmPRs(pairs)
	}
	warm()
	t := time.NewTicker(prTTL - 20*time.Second)
	defer t.Stop()
	for range t.C {
		warm()
	}
}
