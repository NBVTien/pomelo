package core

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/pomelohq/pomelo/internal/services"
)

func (s *Server) handlePorts(w http.ResponseWriter, r *http.Request) {
	type row struct {
		Key   string `json:"key"`
		Ws    string `json:"ws"`
		Svc   string `json:"svc"`
		Port  int    `json:"port"`
		State string `json:"state"`
		Since string `json:"since"`
	}
	rows := []row{}
	for _, l := range services.PortMgr().Snapshot() {
		ws, svc := "", l.Key
		if i := strings.IndexByte(l.Key, '\x1f'); i >= 0 {
			ws, svc = l.Key[:i], l.Key[i+1:]
		}
		rows = append(rows, row{
			Key: l.Key, Ws: ws, Svc: svc, Port: l.Port,
			State: string(l.State), Since: l.Since.UTC().Format(time.RFC3339),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Port < rows[j].Port })

	writeJSON(w, map[string]any{
		"bind_ip": services.BindIP(),
		"range":   map[string]int{"lo": 10000, "hi": 65535},
		"ports":   rows,
		"note":    "Ports are assigned automatically (random, sticky per service, freed when the service dies). Addressing is via the dev-proxy domain, not the raw port.",
	})
}
