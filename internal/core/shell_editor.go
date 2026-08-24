package core

import (
	"encoding/json"
	"github.com/pomelohq/pomelo/internal/provider/shell"
	"net/http"
	"os/exec"

	"github.com/pomelohq/pomelo/internal/services"
)

func (s *Server) handleEditors(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.Editors())
}

func (s *Server) Editors() map[string]any {
	configured := ""
	if s.cfg() != nil && s.cfg().UI != nil {
		configured = s.cfg().UI.Editor
	}
	installed := []string{}
	for _, ed := range []string{"code", "cursor", "zed", "windsurf", "subl"} {
		if _, err := exec.LookPath(ed); err == nil {
			installed = append(installed, ed)
		}
	}
	return map[string]any{"installed": installed, "configured": configured}
}

type openShellReq struct {
	Branch string `json:"branch"`
	IsMain bool   `json:"is_main"`
	Repo   string `json:"repo,omitempty"`
}

func (s *Server) handleOpenShell(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if s.cfg() == nil {
		http.Error(w, "no project config", http.StatusServiceUnavailable)
		return
	}
	var req openShellReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.Branch == "" {
		http.Error(w, "missing branch", http.StatusBadRequest)
		return
	}
	cwd := s.workspaceRoot(req.Branch, req.IsMain)
	label := "ws"
	if req.Repo != "" {
		cwd = repoWorktreePath(s.WorkspaceRoot, req.Repo, req.Branch, req.IsMain)
		label = req.Repo
	}
	holder := shellHolderName(req.Branch, req.IsMain, label)
	if err := services.SpawnHolder(holder, cwd, 0, 0, shell.Interactive()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok": true, "window": holder, "pane_id": "pty:" + holder,
	})
}

type openEditorReq struct {
	Branch      string `json:"branch"`
	IsMain      bool   `json:"is_main"`
	Repo        string `json:"repo,omitempty"`
	Editor      string `json:"editor,omitempty"`
	ResolveOnly bool   `json:"resolve_only,omitempty"`
}

func (s *Server) handleOpenEditor(w http.ResponseWriter, r *http.Request) {
	var req openEditorReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpErr(w, http.StatusBadRequest, "bad json")
		return
	}
	writeJSON(w, s.EditorOpen(req.Branch, req.IsMain, req.Repo, req.Editor, req.ResolveOnly))
}

func (s *Server) EditorOpen(branch string, isMain bool, repo, editor string, resolveOnly bool) map[string]any {
	if s.cfg() == nil {
		return map[string]any{"ok": false, "error": "no project config"}
	}
	var path string
	if repo != "" {
		path = repoWorktreePath(s.WorkspaceRoot, repo, branch, isMain)
	} else {
		path = s.workspaceRoot(branch, isMain)
	}
	configured := ""
	if s.cfg().UI != nil {
		configured = s.cfg().UI.Editor
	}
	if resolveOnly {
		return map[string]any{"ok": true, "path": path, "resolved": true, "editor": configured}
	}
	for _, ed := range []string{editor, configured, "code", "cursor", "zed", "windsurf", "subl"} {
		if ed == "" {
			continue
		}
		bin, err := exec.LookPath(ed)
		if err != nil {
			continue
		}
		c := exec.Command(bin, path)
		if err := c.Start(); err == nil {
			_ = c.Process.Release()
			return map[string]any{"ok": true, "editor": ed, "path": path}
		}
	}
	return map[string]any{"ok": false, "error": "no GUI editor found — install VS Code / Cursor / Zed / Windsurf / Sublime, or set ui.editor"}
}
