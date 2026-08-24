package core

import (
	"net/http"

	"github.com/pomelohq/pomelo/internal/logbuf"
)

func (s *Server) logsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/logs", s.handleLogs)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.LogsData())
}

func (s *Server) LogsData() map[string]any {
	return map[string]any{
		"version": Version,
		"session": s.Project,
		"logfile": logbuf.FilePath(),
		"lines":   logbuf.Lines(),
	}
}
