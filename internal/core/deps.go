package core

import (
	"encoding/json"
	"net/http"
	"os/exec"
)

type depInfo struct {
	Name     string `json:"name"`
	Present  bool   `json:"present"`
	Required bool   `json:"required"`
	Note     string `json:"note"`
}

var checkedDeps = []struct {
	name, bin, note string
	required        bool
}{
	{"git", "git", "per-workspace worktrees", true},
	{"zsh", "zsh", "services launch via zsh so your .zshrc loads", true},
	{"claude", "claude", "the Claude Code agent tabs", false},
	{"docker", "docker", "shared services (Postgres/Redis/…)", false},
}

func (s *Server) handleDeps(w http.ResponseWriter, r *http.Request) {
	deps := make([]depInfo, 0, len(checkedDeps))
	missReq, missOpt := []string{}, []string{}
	for _, d := range checkedDeps {
		_, err := exec.LookPath(d.bin)
		present := err == nil
		deps = append(deps, depInfo{Name: d.name, Present: present, Required: d.required, Note: d.note})
		if !present {
			if d.required {
				missReq = append(missReq, d.name)
			} else {
				missOpt = append(missOpt, d.name)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"deps":             deps,
		"missing_required": missReq,
		"missing_optional": missOpt,
	})
}
