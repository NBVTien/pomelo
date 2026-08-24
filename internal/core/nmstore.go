package core

import (
	"net/http"

	"github.com/pomelohq/pomelo/internal/services"
)

func (s *Server) nmStoreRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/nmstore/list", s.handleNMStoreList)
	mux.HandleFunc("/api/nmstore/delete", s.handleNMStoreDelete)
}

func (s *Server) handleNMStoreList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.NMStoreList())
}

func (s *Server) NMStoreList() map[string]any {
	entries := services.NMStoreEntries()
	branch := ""
	if c := s.cfg(); c != nil {
		branch = c.GlobalDefaultBranch()
	}
	curByRepo := map[string]string{}
	type row struct {
		services.NMStoreEntry
		Current bool `json:"current"`
	}
	out := make([]row, 0, len(entries))
	var total int64
	for _, e := range entries {
		total += e.Bytes
		cur, ok := curByRepo[e.Repo]
		if !ok {
			cur = services.MainLockHash(s.WorkspaceRoot, e.Repo, branch)
			curByRepo[e.Repo] = cur
		}
		out = append(out, row{NMStoreEntry: e, Current: cur != "" && cur == e.Hash})
	}
	return map[string]any{"entries": out, "total": total}
}

func (s *Server) handleNMStoreDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpErr(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	req, err := readJSON[struct {
		Repo string `json:"repo"`
		Hash string `json:"hash"`
	}](r)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "bad json")
		return
	}
	if err := s.NMStoreDelete(req.Repo, req.Hash); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) NMStoreDelete(repo, hash string) error {
	return services.RemoveNMStoreEntry(repo, hash)
}
