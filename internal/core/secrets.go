package core

import (
	"net/http"
	"strings"

	"github.com/pomelohq/pomelo/internal/secrets"
	"github.com/pomelohq/pomelo/internal/services"
)

func (s *Server) secretsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/secrets", s.handleSecretsList)
	mux.HandleFunc("/api/secrets/set", s.handleSecretsSet)
	mux.HandleFunc("/api/secrets/get", s.handleSecretsGet)
}

func (s *Server) handleSecretsGet(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" || strings.HasPrefix(name, "__") {
		httpErr(w, http.StatusBadRequest, "name required")
		return
	}
	v, _ := secrets.Get(s.session(), name)
	writeJSON(w, map[string]any{"name": name, "value": v})
}

func (s *Server) session() string {
	if cfg := s.cfg(); cfg != nil {
		return cfg.Session
	}
	return ""
}

func (s *Server) handleSecretsList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"names": s.SecretNames()})
}

func (s *Server) SecretNames() []string { return secrets.Names(s.session()) }

func (s *Server) SecretGet(name string) string {
	if name == "" || strings.HasPrefix(name, "__") {
		return ""
	}
	v, _ := secrets.Get(s.session(), name)
	return v
}

func (s *Server) SecretSet(name, value string) error {
	if err := secrets.Set(s.session(), name, value); err != nil {
		return err
	}
	if cfg := s.cfg(); cfg != nil && s.WorkspaceRoot != "" {
		go services.RegenerateWorkspaceEnv(s.WorkspaceRoot, cfg, s.DefaultBranch)
	}
	return nil
}

func (s *Server) handleSecretsSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpErr(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	req, err := readJSON[struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}](r)
	if err != nil || req.Name == "" {
		httpErr(w, http.StatusBadRequest, "name required")
		return
	}
	if err := secrets.Set(s.session(), req.Name, req.Value); err != nil {
		httpErr(w, http.StatusInternalServerError, "%s", err.Error())
		return
	}
	if cfg := s.cfg(); cfg != nil && s.WorkspaceRoot != "" {
		go services.RegenerateWorkspaceEnv(s.WorkspaceRoot, cfg, s.DefaultBranch)
	}
	writeJSON(w, map[string]any{"ok": true})
}
