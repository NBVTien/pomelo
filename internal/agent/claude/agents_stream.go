package claude

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pomelohq/pomelo/internal/paths"
	"github.com/pomelohq/pomelo/internal/ptyhost"
	"github.com/pomelohq/pomelo/internal/services"
)

type agentEvent struct {
	Branch  string `json:"branch"`
	IsMain  bool   `json:"is_main"`
	Service string `json:"service"`
	State   string `json:"state"`
	Prev    string `json:"prev"`
}

func (s *Feature) broadcastAgent(ev agentEvent) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	key := wsStateKey(ev.Branch, ev.IsMain)
	s.agentStateMu.Lock()
	s.agentState[key] = ev.State
	s.agentStateMu.Unlock()
	s.agentMu.Lock()
	defer s.agentMu.Unlock()
	for ch := range s.agentSubs {
		select {
		case ch <- data:
		default:
		}
	}
}

func (s *Feature) handleAgentStates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"states": s.AgentStates()})
}

func (s *Feature) AgentStates() map[string]string {
	s.agentStateMu.Lock()
	out := make(map[string]string, len(s.agentState))
	for k, v := range s.agentState {
		out[k] = v
	}
	s.agentStateMu.Unlock()

	sess := ""
	if c := s.cfg(); c != nil {
		sess = c.Session
	}
	dir := paths.StatePath("agents")
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(dir, e.Name())
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var f struct{ Branch, State string }
			if json.Unmarshal(b, &f) != nil || f.Branch == "" {
				continue
			}
			key := "ws:" + f.Branch
			if c := s.cfg(); c != nil && f.Branch == c.GlobalDefaultBranch() {
				key = "main:" + f.Branch
			}
			if !ptyhost.HolderAlive(services.WsServiceHolderName(sess, f.Branch, "claude-raw")) {
				_ = os.Remove(path)
				delete(out, key)
				continue
			}
			out[key] = f.State
		}
	}
	return out
}

func (s *Feature) handleAgentStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan []byte, 16)
	s.agentMu.Lock()
	s.agentSubs[ch] = struct{}{}
	s.agentMu.Unlock()
	defer func() {
		s.agentMu.Lock()
		delete(s.agentSubs, ch)
		s.agentMu.Unlock()
	}()

	ka := time.NewTicker(25 * time.Second)
	defer ka.Stop()
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-ch:
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(data)
			_, _ = w.Write([]byte("\n\n"))
			flusher.Flush()
		case <-ka.C:
			_, _ = w.Write([]byte(": keepalive\n\n"))
			flusher.Flush()
		}
	}
}
