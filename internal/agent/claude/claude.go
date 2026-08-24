package claude

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/pomelohq/pomelo/internal/httpx"
	"github.com/pomelohq/pomelo/internal/provider/shell"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pomelohq/pomelo/internal/agent/codeagent"
	"github.com/pomelohq/pomelo/internal/ptyhost"
	"github.com/pomelohq/pomelo/internal/services"
)

func timeNowNano() int64 { return time.Now().UnixNano() }

func (s *Feature) handleClaudeSession(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cwd := q.Get("cwd")
	if cwd == "" {
		http.Error(w, "missing cwd", http.StatusBadRequest)
		return
	}
	root := claudeProjectsRoot()
	if root == "" {
		http.Error(w, "no $HOME", http.StatusInternalServerError)
		return
	}
	dir := filepath.Join(root, encodeClaudeProjectDir(cwd))
	if branch := q.Get("branch"); branch != "" {
		id := deterministicSessionID("chat:" + services.WsKey(branch, q.Get("is_main") == "true"))
		p := filepath.Join(dir, id+".jsonl")
		var sess *Session
		if info, err := os.Stat(p); err == nil {
			sess = &Session{ID: id, Path: p, Project: filepath.Base(dir), Modified: info.ModTime(), SizeKB: info.Size() / 1024}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"session": sess})
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"session": nil})
		return
	}
	var newest *Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if newest == nil || info.ModTime().After(newest.Modified) {
			newest = &Session{
				ID:       strings.TrimSuffix(e.Name(), ".jsonl"),
				Path:     filepath.Join(dir, e.Name()),
				Project:  filepath.Base(dir),
				Modified: info.ModTime(),
				SizeKB:   info.Size() / 1024,
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"session": newest})
}

func (s *Feature) handleClaudeUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Data      string `json:"data"`
		MediaType string `json:"media_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	raw := req.Data
	if i := strings.Index(raw, ","); strings.HasPrefix(raw, "data:") && i >= 0 {
		raw = raw[i+1:]
	}
	bytes, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		http.Error(w, "bad base64", http.StatusBadRequest)
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "no $HOME", http.StatusInternalServerError)
		return
	}
	dir := filepath.Join(home, ".claude", "image-cache", "web")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, "mkdir: "+err.Error(), http.StatusInternalServerError)
		return
	}
	ext := "png"
	switch req.MediaType {
	case "image/jpeg", "image/jpg":
		ext = "jpg"
	case "image/gif":
		ext = "gif"
	case "image/webp":
		ext = "webp"
	}
	name := fmt.Sprintf("paste-%d.%s", timeNowNano(), ext)
	abs := filepath.Join(dir, name)
	if err := os.WriteFile(abs, bytes, 0o644); err != nil {
		http.Error(w, "write: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"path": abs})
}

func (s *Feature) handleClaudeImage(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "no $HOME", http.StatusInternalServerError)
		return
	}
	root := filepath.Join(home, ".claude", "image-cache")
	clean := filepath.Clean(p)
	if clean != root && !strings.HasPrefix(clean, root+string(os.PathSeparator)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	http.ServeFile(w, r, clean)
}

func (s *Feature) handleClaudeTerminal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.Err(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	req, err := httpx.Read[struct {
		Branch string `json:"branch"`
		IsMain bool   `json:"is_main"`
	}](r)
	if err != nil || req.Branch == "" {
		httpx.Err(w, http.StatusBadRequest, "missing branch")
		return
	}
	httpx.Write(w, s.ClaudeTerminal(req.Branch, req.IsMain))
}

func (s *Feature) ClaudeTerminal(branch string, isMain bool) map[string]any {
	if s.cfg() == nil {
		return map[string]any{"error": "no project config"}
	}
	key := services.WsKey(branch, isMain)
	s.hdMu.Lock()
	d := s.headless[key]
	s.hdMu.Unlock()
	if d != nil {
		d.mu.Lock()
		running := d.running
		d.mu.Unlock()
		if running {
			return map[string]any{"error": "Claude is working — wait for the turn to finish, then open the CLI"}
		}
	}

	cwd := s.workspaceRoot(branch, isMain)
	s.writeWorkspaceMap(branch, isMain)
	sid := deterministicSessionID("chat:" + key)
	bin := ResolveClaudeBin()
	sessionFlag := "--session-id"
	if root := claudeProjectsRoot(); root != "" {
		if _, err := os.Stat(filepath.Join(root, encodeClaudeProjectDir(cwd), sid+".jsonl")); err == nil {
			sessionFlag = "--resume"
		}
	}
	flags := []string{
		sessionFlag, sid,
		"--mcp-config", shell.Quote(s.mcpConfigJSON(branch)),
		"--add-dir", shell.Quote(imageCacheDir()),
		"--append-system-prompt", shell.Quote(s.chatSystemPrompt(branch))}
	flagStr := strings.Join(flags, " ")
	pathExport := ""
	if p := services.ToolPath(); p != "" {
		pathExport = "export PATH=" + shell.Quote(p) + "; "
	}
	launch := pathExport + "export TERM=xterm-256color COLORTERM=truecolor; unsetopt monitor 2>/dev/null; cd " +
		shell.Quote(cwd) + " && exec " + shell.Quote(bin) + " " + flagStr

	holder := services.WsServiceHolderName(s.cfg().Session, branch, "claude-raw")
	if !ptyhost.HolderAlive(holder) {
		if err := services.SpawnHolder(holder, cwd, 0, 0, shell.Command(launch)); err != nil {
			return map[string]any{"error": err.Error()}
		}
		if c, err := ptyhost.DialWait(ptyhost.SocketPath(holder), 5*time.Second); err == nil {
			_ = c.Close()
		}
	}
	return map[string]any{"pane_id": "pty:" + holder, "window": holder}
}

func (s *Feature) claudeServiceName() string {
	for _, svcName := range s.cfg().WsServiceOrder {
		if agent := codeagent.LookupAgent(svcName); agent != nil {
			return svcName
		}
	}
	return ""
}
