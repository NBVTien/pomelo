package core

import (
	"net/http"

	"github.com/pomelohq/pomelo/internal/appstate"
)

func (s *Server) syncConfigRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/sync/config", s.handleSyncConfig)
}

func (s *Server) handleSyncConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		req, err := readJSON[struct {
			RefreshMain        bool `json:"refresh_main"`
			RefreshIntervalSec int  `json:"refresh_interval_sec"`
		}](r)
		if err != nil {
			httpErr(w, http.StatusBadRequest, "bad json")
			return
		}
		if err := s.SyncSet(req.RefreshMain, req.RefreshIntervalSec); err != nil {
			httpErr(w, http.StatusInternalServerError, "%s", err.Error())
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	writeJSON(w, s.SyncGet())
}

func (s *Server) SyncGet() map[string]any {
	rm, iv := s.effectiveSync()
	if iv == 0 {
		iv = 1800
	}
	return map[string]any{"refresh_main": rm, "refresh_interval_sec": iv}
}

func (s *Server) SyncSet(refreshMain bool, intervalSec int) error {
	st := appstate.Load(s.session())
	st.Sync = appstate.SyncConfig{Configured: true, RefreshMain: refreshMain, RefreshIntervalSec: intervalSec}
	return appstate.Save(s.session(), st)
}
