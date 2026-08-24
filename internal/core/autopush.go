package core

import (
	"time"

	"github.com/pomelohq/pomelo/internal/config"
	"github.com/pomelohq/pomelo/internal/services"
)

func (s *Server) autoPushLoop() {
	if s.WorkspaceRoot == "" {
		return
	}
	for {
		cfg := s.cfg()
		if cfg == nil || cfg.Sync == nil || !cfg.Sync.AutoPush {
			time.Sleep(60 * time.Second)
			continue
		}
		s.autoPushOnce(cfg)
		time.Sleep(syncInterval(cfg))
	}
}

func syncInterval(cfg *config.Config) time.Duration {
	n := 180
	if cfg.Sync != nil && cfg.Sync.IntervalSec > 0 {
		n = cfg.Sync.IntervalSec
	}
	if n < 30 {
		n = 30
	}
	return time.Duration(n) * time.Second
}

var lastWIPPrune time.Time

func (s *Server) autoPushOnce(cfg *config.Config) {
	known := make(map[string]bool, len(cfg.Repos))
	for n := range cfg.Repos {
		known[n] = true
	}
	repoWt := map[string]string{}
	for _, ws := range scanWorkspaces(s.WorkspaceRoot, s.DefaultBranch, known, true) {
		if ws.IsMain {
			continue
		}
		for _, repo := range ws.Repos {
			wt := repoWorktreePath(s.WorkspaceRoot, repo.Name, ws.Branch, ws.IsMain)
			if st, err := osStat(wt); err != nil || !st.IsDir() {
				continue
			}
			repoWt[repo.Name] = wt
			services.PushWorktree(wt)
		}
	}
	if time.Since(lastWIPPrune) > time.Hour {
		lastWIPPrune = time.Now()
		for _, wt := range repoWt {
			services.PruneWIPRefs(wt)
		}
	}
}
