package core

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pomelohq/pomelo/internal/ptyhost"
	"github.com/pomelohq/pomelo/internal/services"
)

type liveTerminal struct {
	PaneID  string `json:"pane_id"`
	Window  string `json:"window"`
	Cwd     string `json:"cwd"`
	Command string `json:"command"`
}

func (s *Server) handleTerminals(w http.ResponseWriter, r *http.Request) {
	if s.cfg() == nil {
		http.Error(w, "no project config", http.StatusServiceUnavailable)
		return
	}
	branch := r.URL.Query().Get("branch")
	isMain := r.URL.Query().Get("main") == "1"
	wsDir := ""
	if branch != "" {
		wsDir = filepath.Clean(s.workspaceRoot(branch, isMain))
	}
	terms := []liveTerminal{}
	if branch != "" {
		prefix := "sh-" + services.WsKey(branch, isMain) + "-"
		for _, h := range ptyhost.Holders() {
			if strings.HasPrefix(h.Name, prefix) {
				terms = append(terms, liveTerminal{PaneID: "pty:" + h.Name, Window: "", Cwd: wsDir, Command: "sh (native)"})
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"terminals": terms})
}

func (s *Server) handleServicePeek(w http.ResponseWriter, r *http.Request) {
	if s.cfg() == nil {
		http.Error(w, "no project config", http.StatusServiceUnavailable)
		return
	}
	window := r.URL.Query().Get("window")
	if window == "" {
		http.Error(w, "missing window", http.StatusBadRequest)
		return
	}
	lines := 10
	if n := r.URL.Query().Get("lines"); n != "" {
		if v, err := strconv.Atoi(n); err == nil && v > 0 && v <= 60 {
			lines = v
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if !ptyhost.HolderAlive(window) {
		crashed, out := ptyhost.CrashInfo(window)
		_ = json.NewEncoder(w).Encode(map[string]any{"running": false, "crashed": crashed, "lines": linesTail(out, lines)})
		return
	}
	tail := snapshotTail(window, lines)
	_ = json.NewEncoder(w).Encode(map[string]any{"running": true, "lines": tail})
}

var ansiRE = regexp.MustCompile("\x1b\\][^\x07]*(\x07|\x1b\\\\)|\x1b[\\[][0-9;?]*[ -/]*[@-~]|\x1b[@-Z\\\\-_]")

func snapshotTail(holder string, n int) []string {
	return linesTail(ptyhost.Snapshot(holder, 150*time.Millisecond), n)
}

func linesTail(raw []byte, n int) []string {
	if len(raw) == 0 {
		return []string{}
	}
	clean := ansiRE.ReplaceAllString(string(raw), "")
	clean = strings.ReplaceAll(clean, "\r", "")
	all := strings.Split(clean, "\n")
	out := make([]string, 0, len(all))
	for _, l := range all {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return out
}

func (s *Server) handlePeekAll(w http.ResponseWriter, r *http.Request) {
	if s.cfg() == nil {
		http.Error(w, "no project config", http.StatusServiceUnavailable)
		return
	}
	lines := 8
	if n := r.URL.Query().Get("lines"); n != "" {
		if v, err := strconv.Atoi(n); err == nil && v > 0 && v <= 40 {
			lines = v
		}
	}
	var windows []string
	if q := r.URL.Query().Get("windows"); q != "" {
		windows = strings.Split(q, ",")
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"windows": s.PeekWindows(windows, lines)})
}

func (s *Server) PeekWindows(windows []string, lines int) map[string][]string {
	result := map[string][]string{}
	for _, wn := range windows {
		holder := strings.TrimSpace(wn)
		if holder != "" && ptyhost.HolderAlive(holder) {
			result[holder] = snapshotTail(holder, lines)
		}
	}
	return result
}
