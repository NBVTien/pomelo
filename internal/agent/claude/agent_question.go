package claude

import (
	"bufio"
	"encoding/json"
	"github.com/pomelohq/pomelo/internal/httpx"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s *Feature) handleAgentQuestion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.Err(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	req, err := httpx.Read[struct {
		Items []struct {
			Branch string `json:"branch"`
			IsMain bool   `json:"is_main"`
			Path   string `json:"path"`
		} `json:"items"`
	}](r)
	if err != nil {
		httpx.Err(w, http.StatusBadRequest, "bad json")
		return
	}
	type question struct {
		Text string `json:"text"`
	}
	out := map[string]question{}
	for _, it := range req.Items {
		if it.Path == "" {
			continue
		}
		text := lastAssistantText(NewestTranscript(it.Path))
		if text == "" {
			continue
		}
		key := "ws:" + it.Branch
		if it.IsMain {
			key = "main:" + it.Branch
		}
		out[key] = question{Text: text}
	}
	httpx.Write(w, map[string]any{"questions": out})
}

func NewestTranscript(cwd string) string {
	dir := filepath.Join(claudeProjectsRoot(), encodeClaudeProjectDir(cwd))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	best, bestMod := "", int64(0)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if m := info.ModTime().UnixNano(); m > bestMod {
			best, bestMod = filepath.Join(dir, e.Name()), m
		}
	}
	return best
}

const questionTailBytes = 256 * 1024

func lastAssistantText(path string) string {
	if path == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.Size() > questionTailBytes {
		if _, err := f.Seek(info.Size()-questionTailBytes, io.SeekStart); err != nil {
			return ""
		}
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	last := ""
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			first = false
			if !strings.HasPrefix(line, "{") {
				continue
			}
		}
		if t := assistantText(line); t != "" {
			last = t
		}
	}
	return truncateRunes(last, 400)
}

func assistantText(line string) string {
	var ev struct {
		Type    string `json:"type"`
		Message struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal([]byte(line), &ev) != nil || ev.Type != "assistant" {
		return ""
	}
	var parts []string
	for _, c := range ev.Message.Content {
		if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
			parts = append(parts, strings.TrimSpace(c.Text))
		}
	}
	return strings.Join(parts, "\n")
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
