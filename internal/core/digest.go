package core

import (
	"fmt"
	"net/http"
	"os"

	"strconv"
	"strings"
	"time"

	"github.com/pomelohq/pomelo/internal/agent/claude"
	"github.com/pomelohq/pomelo/internal/services"
)

type repoDigest struct {
	Repo    string `json:"repo"`
	Alias   string `json:"alias"`
	Commits int    `json:"commits"`
	Latest  string `json:"latest"`
}

func (s *Server) handleWorkspaceDigest(w http.ResponseWriter, r *http.Request) {
	if s.cfg() == nil {
		httpErr(w, http.StatusServiceUnavailable, "no project config")
		return
	}
	branch := r.URL.Query().Get("branch")
	isMain := r.URL.Query().Get("is_main") == "true"
	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	if branch == "" || since <= 0 {
		httpErr(w, http.StatusBadRequest, "missing branch/since")
		return
	}
	sinceArg := fmt.Sprintf("--since=@%d", since)

	repos := []repoDigest{}
	total := 0
	for _, dirName := range s.cfg().RepoOrder {
		dir := s.cfg().Repos[dirName]
		if dir == nil {
			continue
		}
		alias := dir.Alias
		if alias == "" {
			alias = dirName
		}
		wt := services.RepoWorktreePath(s.WorkspaceRoot, dirName, branch, isMain)
		if !dirExists(wt) {
			continue
		}
		out, err := services.RunTimeout(4*time.Second, wt, "git", "log", sinceArg, "--format=%s")
		if err != nil {
			continue
		}
		lines := []string{}
		for _, ln := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if strings.TrimSpace(ln) != "" {
				lines = append(lines, ln)
			}
		}
		if len(lines) == 0 {
			continue
		}
		repos = append(repos, repoDigest{Repo: dirName, Alias: alias, Commits: len(lines), Latest: lines[0]})
		total += len(lines)
	}

	agentActive, agentAt := transcriptActivity(services.WorkspaceRootDir(s.WorkspaceRoot, branch, isMain), since)

	writeJSON(w, map[string]any{
		"repos":         repos,
		"total_commits": total,
		"agent_active":  agentActive,
		"agent_at":      agentAt,
	})
}

func transcriptActivity(cwd string, since int64) (bool, int64) {
	path := claude.NewestTranscript(cwd)
	if path == "" {
		return false, 0
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, 0
	}
	mod := info.ModTime().Unix()
	return mod > since, mod
}
