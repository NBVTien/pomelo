package claude

import (
	"bytes"
	"encoding/json"
	"github.com/pomelohq/pomelo/internal/httpx"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func validTranscriptPath(path string) (string, bool) {
	root := claudeProjectsRoot()
	abs, err := filepath.Abs(path)
	if err != nil || root == "" {
		return "", false
	}
	if !strings.HasPrefix(abs, root+string(filepath.Separator)) || !strings.HasSuffix(abs, ".jsonl") {
		return "", false
	}
	return abs, true
}

func splitCompleteLines(data []byte) (lines [][]byte, consumed int) {
	i := 0
	for {
		j := bytes.IndexByte(data[i:], '\n')
		if j < 0 {
			break
		}
		lines = append(lines, data[i:i+j+1])
		i += j + 1
	}
	return lines, i
}

func claudeProjectsRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "projects")
}

func encodeClaudeProjectDir(cwd string) string {
	cleaned := filepath.Clean(cwd)
	return strings.NewReplacer(string(filepath.Separator), "-").Replace(cleaned)
}

type Session struct {
	ID       string    `json:"id"`
	Path     string    `json:"path"`
	Project  string    `json:"project"`
	Modified time.Time `json:"modified"`
	SizeKB   int64     `json:"size_kb"`
}

func (s *Feature) handleSessions(w http.ResponseWriter, r *http.Request) {
	root := claudeProjectsRoot()
	if root == "" {
		http.Error(w, "no $HOME", http.StatusInternalServerError)
		return
	}
	cwd := r.URL.Query().Get("cwd")
	var dirs []string
	if cwd != "" {
		dirs = append(dirs, filepath.Join(root, encodeClaudeProjectDir(cwd)))
	} else {
		entries, _ := os.ReadDir(root)
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, filepath.Join(root, e.Name()))
			}
		}
	}
	var out []Session
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			out = append(out, Session{
				ID:       strings.TrimSuffix(e.Name(), ".jsonl"),
				Path:     filepath.Join(d, e.Name()),
				Project:  filepath.Base(d),
				Modified: info.ModTime(),
				SizeKB:   info.Size() / 1024,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Modified.After(out[j].Modified) })
	if len(out) > 30 {
		out = out[:30]
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"sessions": out})
}

func (s *Feature) handleTranscriptRange(w http.ResponseWriter, r *http.Request) {
	abs, ok := validTranscriptPath(r.URL.Query().Get("path"))
	if !ok {
		httpx.Err(w, http.StatusForbidden, "path must point inside ~/.claude/projects/*.jsonl")
		return
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		httpx.Err(w, http.StatusNotFound, "%s", err.Error())
		return
	}
	lines, _ := splitCompleteLines(data)
	total := len(lines)
	start, _ := strconv.Atoi(r.URL.Query().Get("start"))
	end, _ := strconv.Atoi(r.URL.Query().Get("end"))
	if end <= 0 || end > total {
		end = total
	}
	if start < 0 {
		start = 0
	}
	if start > end {
		start = end
	}
	out := make([]string, 0, end-start)
	for _, ln := range lines[start:end] {
		out = append(out, string(ln))
	}
	httpx.Write(w, map[string]any{"lines": out, "total": total, "start": start})
}
