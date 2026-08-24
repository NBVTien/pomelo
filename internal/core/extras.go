package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pomelohq/pomelo/internal/services"
)

type envSetReq struct {
	Branch string `json:"branch"`
	IsMain bool   `json:"is_main"`
	Repo   string `json:"repo"`
	Svc    string `json:"svc,omitempty"`
	Env    string `json:"env"`
}

func (s *Server) handleEnvSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if s.cfg() == nil {
		http.Error(w, "no project config", http.StatusServiceUnavailable)
		return
	}
	var req envSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	writeJSON(w, s.EnvSet(req.Branch, req.IsMain, req.Repo, req.Svc, req.Env))
}

func (s *Server) EnvSet(branch string, isMain bool, repo, svc, env string) map[string]any {
	if s.cfg() == nil {
		return map[string]any{"ok": false, "error": "no project config"}
	}
	envName := env
	if envName == "" {
		envName = "local"
	}
	if envName != "local" {
		if err := s.cfg().ValidateEnvironment(envName); err != nil {
			return map[string]any{"ok": false, "error": err.Error()}
		}
	}
	dir, ok := s.cfg().Repos[repo]
	if !ok || svc == "" {
		return map[string]any{"ok": false, "error": "repo and svc required"}
	}
	alias := dir.Alias
	if alias == "" {
		alias = repo
	}
	key := alias
	if sc, ok := dir.Services[svc]; ok && sc.Dir != "" {
		key = alias + "/" + svc
	}
	wsFolder := filepath.Join(s.WorkspaceRoot, "workspace--"+branch)
	if isMain {
		wsFolder = filepath.Join(s.WorkspaceRoot, "workspace--"+s.cfg().GlobalDefaultBranch())
	}
	state := services.LoadWorkspaceState(wsFolder)
	if state.ServiceEnvs == nil {
		state.ServiceEnvs = make(map[string]string)
	}
	state.ServiceEnvs[key] = envName
	services.SaveWorkspaceState(wsFolder, &state)
	services.RegenerateWorkspaceEnv(s.WorkspaceRoot, s.cfg(), branch)
	return map[string]any{"ok": true}
}

func (s *Server) handleRepos(w http.ResponseWriter, r *http.Request) {
	if s.cfg() == nil {
		http.Error(w, "no project config", http.StatusServiceUnavailable)
		return
	}
	var names []string
	for n := range s.cfg().Repos {
		names = append(names, n)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"repos": names})
}

func (s *Server) handleEnvironments(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.Environments())
}

func (s *Server) Environments() map[string]any {
	if s.cfg() == nil {
		return map[string]any{"environments": []string{}}
	}
	var names []string
	for n := range s.cfg().Environments {
		names = append(names, n)
	}
	return map[string]any{"environments": names}
}

type dbReq struct {
	Branch string `json:"branch"`
	IsMain bool   `json:"is_main"`
}

func (s *Server) handleDBReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if s.cfg() == nil {
		http.Error(w, "no project config", http.StatusServiceUnavailable)
		return
	}
	var req dbReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	dbNames := s.databasesFor(req.Branch)
	if len(dbNames) == 0 {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "dropped": 0, "created": []string{}})
		return
	}
	host, port, user, pw := s.pgConn()
	services.DropSharedDBsBatch(host, port, dbNames, user, pw)
	results := services.CreateSharedDBsBatch(host, port, dbNames, user, pw)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok": true, "dropped": len(dbNames), "created": results,
	})
}

func (s *Server) databasesFor(branch string) []string {
	var out []string
	for _, dir := range s.cfg().Repos {
		for _, tpl := range dir.Databases {
			out = append(out, s.cfg().Session+"_"+services.ResolveBranchTokens(tpl, branch))
		}
	}
	return out
}

func (s *Server) pgConn() (host string, port uint16, user, pw string) {
	host = s.cfg().SharedHost("postgres")
	port = uint16(services.SharedPort("postgres"))
	if port == 0 {
		port = 5432
	}
	user, pw = "postgres", "postgres"
	if pg, ok := s.cfg().SharedServices["postgres"]; ok {
		if pg.DBUser != "" {
			user = pg.DBUser
		}
		if pg.DBPassword != "" {
			pw = pg.DBPassword
		}
	}
	return
}

type gitOpReq struct {
	Branch       string `json:"branch"`
	IsMain       bool   `json:"is_main"`
	Repo         string `json:"repo"`
	TargetBranch string `json:"target_branch,omitempty"`
}

func (s *gitFeature) gitInRepo(req gitOpReq, args ...string) ([]byte, error) {
	if s.WorkspaceRoot == "" {
		return nil, fmt.Errorf("workspace root unknown")
	}
	wt := repoWorktreePath(s.WorkspaceRoot, req.Repo, req.Branch, req.IsMain)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	full := append([]string{"-C", wt}, args...)
	return exec.CommandContext(ctx, "git", full...).CombinedOutput()
}

func (s *gitFeature) handleGitStatus(w http.ResponseWriter, r *http.Request) {
	s.gitRun(w, r, "status", "--short", "--branch")
}
func (s *gitFeature) handleGitPull(w http.ResponseWriter, r *http.Request) {
	s.gitRun(w, r, "pull", "--ff-only")
}

func (s *gitFeature) Pull(branch, repo string, isMain bool) map[string]any {
	out, err := s.gitInRepo(gitOpReq{Branch: branch, Repo: repo, IsMain: isMain}, "pull", "--ff-only")
	return map[string]any{"ok": err == nil, "output": string(out), "error": errStr(err)}
}
func (s *gitFeature) handleGitDiff(w http.ResponseWriter, r *http.Request) {
	s.gitRun(w, r, "diff", "--stat")
}
func (s *gitFeature) handleGitCheckout(w http.ResponseWriter, r *http.Request) {
	var req gitOpReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.TargetBranch == "" {
		http.Error(w, "target_branch required", http.StatusBadRequest)
		return
	}
	out, err := s.gitInRepo(req, "checkout", req.TargetBranch)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": false, "output": string(out), "error": err.Error(),
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "output": string(out)})
}

func (s *gitFeature) gitRun(w http.ResponseWriter, r *http.Request, args ...string) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req gitOpReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	out, err := s.gitInRepo(req, args...)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok": err == nil, "output": string(out),
		"error": errStr(err),
	})
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type gitBranchesReq struct {
	Branch string `json:"branch"`
	IsMain bool   `json:"is_main"`
	Repo   string `json:"repo"`
}

func (s *gitFeature) handleGitBranches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req gitBranchesReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	out, err := s.gitInRepo(gitOpReq{Branch: req.Branch, IsMain: req.IsMain, Repo: req.Repo},
		"for-each-ref", "--format=%(refname:short)", "refs/heads", "refs/remotes")
	if err != nil {
		http.Error(w, string(out), http.StatusInternalServerError)
		return
	}
	var branches []string
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, "/HEAD") {
			continue
		}
		short := strings.TrimPrefix(line, "origin/")
		if seen[short] {
			continue
		}
		seen[short] = true
		branches = append(branches, short)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"branches": branches})
}
