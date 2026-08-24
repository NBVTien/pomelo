package services

import (
	"os"
	"strings"

	"github.com/pomelohq/pomelo/internal/config"
)

type ServiceStatus struct {
	Repo string `json:"repo"`
	Name string `json:"name"`
	Up   bool   `json:"up"`
	Port int    `json:"port,omitempty"`
}

type WorkspaceStatus struct {
	Branch       string          `json:"branch"`
	IsMain       bool            `json:"is_main"`
	Path         string          `json:"path"`
	Phase        string          `json:"phase"`
	Ready        int             `json:"ready"`
	Total        int             `json:"total"`
	Services     []ServiceStatus `json:"services"`
	MissingRepos []string        `json:"missing_repos,omitempty"`
}

func ScanWorkspaceStatus(configDir string, cfg *config.Config) []WorkspaceStatus {
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return nil
	}
	var out []WorkspaceStatus
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		branch, ok := strings.CutPrefix(e.Name(), "workspace--")
		if !ok {
			continue
		}
		if st, found := WorkspaceStatusFor(configDir, cfg, branch); found {
			out = append(out, *st)
		}
	}
	return out
}

func WorkspaceStatusFor(configDir string, cfg *config.Config, branch string) (*WorkspaceStatus, bool) {
	isMain := branch == cfg.GlobalDefaultBranch()
	root := WorkspaceRootDir(configDir, branch, isMain)
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return nil, false
	}
	wsKey := WsKey(branch, isMain)
	ws := &WorkspaceStatus{Branch: branch, IsMain: isMain, Path: root}
	for _, dirName := range cfg.RepoOrder {
		dir := cfg.Repos[dirName]
		if dir == nil || !dir.HasWorktreeConfig() {
			continue
		}
		if fi, err := os.Stat(RepoWorktreePath(configDir, dirName, branch, isMain)); err != nil || !fi.IsDir() {
			ws.MissingRepos = append(ws.MissingRepos, dirName)
			continue
		}
		alias := dir.Alias
		if alias == "" {
			alias = dirName
		}
		for _, svcName := range dir.ServiceOrder {
			up := ServiceRunning(cfg.Session, branch, dirName, svcName)
			ws.Services = append(ws.Services, ServiceStatus{
				Repo: dirName, Name: svcName, Up: up,
				Port: Port(configDir, wsKey, alias+"~"+svcName),
			})
			ws.Total++
			if up {
				ws.Ready++
			}
		}
	}
	ws.Phase = workspacePhase(ws.Ready, ws.Total)
	return ws, true
}

func workspacePhase(ready, total int) string {
	switch {
	case total == 0:
		return "Empty"
	case ready == 0:
		return "Stopped"
	case ready < total:
		return "Partial"
	default:
		return "Running"
	}
}
