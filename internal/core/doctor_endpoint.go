package core

import (
	"net/http"

	"github.com/pomelohq/pomelo/internal/doctor"
)

func (s *Server) doctorRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/config/doctor", s.handleConfigDoctor)
}

func (s *Server) handleConfigDoctor(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.DoctorReport())
}

func (s *Server) DoctorReport() map[string]any {
	findings := doctor.Diagnose(s.cfg(), s.WorkspaceRoot, s.session())
	errors, warns := 0, 0
	for _, f := range findings {
		switch f.Severity {
		case doctor.SevError:
			errors++
		case doctor.SevWarn:
			warns++
		}
	}
	return map[string]any{"findings": findings, "errors": errors, "warnings": warns}
}
