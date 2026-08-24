package core

import (
	"net/http"
	"strconv"

	"github.com/pomelohq/pomelo/internal/services"
)

func (s *Server) handleNetwork(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.NetworkInfo())
}

func (s *Server) NetworkInfo() map[string]any {
	cfg := s.cfg()
	if cfg == nil {
		return map[string]any{"error": "no project"}
	}
	const domain = "localhost"
	proxyPort := s.webPort() + 2
	whPort := s.webPort() + 1
	return map[string]any{
		"bind_ip":    services.BindIP(),
		"domain":     domain,
		"proxy_port": proxyPort,
		"proxy_url":  "http://<service>.<repo>.<branch>." + domain + ":" + strconv.Itoa(proxyPort),
		"webhook":    map[string]any{"configured": true, "enabled": true, "listen_port": whPort},
	}
}

func (s *Server) NetworkSetPorts(proxyPort, webhookPort int) map[string]any {
	return map[string]any{"ok": true}
}
