package core

import (
	"net/http"

	"github.com/pomelohq/pomelo/internal/plugin"
)

type gitFeature struct {
	WorkspaceRoot string
}

func (*gitFeature) Name() string { return "git" }

func (s *gitFeature) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/api/git/status", s.handleGitStatus)
	mux.HandleFunc("/api/git/pull", s.handleGitPull)
	mux.HandleFunc("/api/git/diff", s.handleGitDiff)
	mux.HandleFunc("/api/git/checkout", s.handleGitCheckout)
	mux.HandleFunc("/api/git/branches", s.handleGitBranches)
}

func (s *Server) GitPull(branch, repo string, isMain bool) map[string]any {
	return s.git.Pull(branch, repo, isMain)
}

var _ plugin.HTTPProvider = (*gitFeature)(nil)
