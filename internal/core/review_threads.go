package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pomelohq/pomelo/internal/services"
)

// A review note is a private annotation anchored to a repo/path/line-range in a
// workspace review — used to flag or question the AI's change while verifying it.
// Notes are workspace-scoped and stored next to the review artifact at
// <root>/.pom/reviews/<branch>.threads.json.
type reviewComment struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
}

type reviewThread struct {
	ID        string          `json:"id"`
	Repo      string          `json:"repo"`
	Path      string          `json:"path"`
	Start     int             `json:"start"`
	End       int             `json:"end"`
	Side      string          `json:"side"`
	Resolved  bool            `json:"resolved"`
	Source    string          `json:"source"` // "local" (reserved for future kinds)
	CreatedAt string          `json:"createdAt"`
	Comments  []reviewComment `json:"comments"`
}

func (s *Server) threadsPath(branch string) (string, bool) {
	if s.WorkspaceRoot == "" {
		return "", false
	}
	name := services.BranchSafe(branch)
	if name == "" {
		return "", false
	}
	return filepath.Join(s.WorkspaceRoot, ".pom", "reviews", name+".threads.json"), true
}

func (s *Server) loadThreads(branch string) []reviewThread {
	p, ok := s.threadsPath(branch)
	if !ok {
		return nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var wrap struct {
		Threads []reviewThread `json:"threads"`
	}
	if json.Unmarshal(b, &wrap) != nil {
		return nil
	}
	return wrap.Threads
}

func (s *Server) saveThreads(branch string, threads []reviewThread) error {
	p, ok := s.threadsPath(branch)
	if !ok {
		return fmt.Errorf("no workspace")
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, _ := json.Marshal(map[string]any{"threads": threads})
	return os.WriteFile(p, b, 0o644)
}

// ReviewThreads returns the workspace's local review notes. These are private to
// the person verifying the AI's change (flag a spot, jot why) — not shared anywhere.
func (s *Server) ReviewThreads(branch string, isMain bool) []byte {
	threads := s.loadThreads(branch)
	sort.SliceStable(threads, func(i, j int) bool {
		if threads[i].Path != threads[j].Path {
			return threads[i].Path < threads[j].Path
		}
		return threads[i].Start < threads[j].Start
	})
	b, _ := json.Marshal(map[string]any{"threads": threads})
	return b
}

type reviewThreadAddReq struct {
	Branch string `json:"branch"`
	IsMain bool   `json:"is_main"`
	Repo   string `json:"repo"`
	Path   string `json:"path"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Side   string `json:"side"`
	Body   string `json:"body"`
	Author string `json:"author"`
}

func (s *Server) ReviewThreadAdd(req reviewThreadAddReq) map[string]any {
	if strings.TrimSpace(req.Body) == "" {
		return map[string]any{"ok": false, "error": "empty comment"}
	}
	if req.Repo == "" || req.Path == "" {
		return map[string]any{"ok": false, "error": "missing repo/path"}
	}
	author := req.Author
	if author == "" {
		author = "you"
	}
	if req.Side == "" {
		req.Side = "head"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	t := reviewThread{
		ID: fmt.Sprintf("t%d", time.Now().UnixNano()), Repo: req.Repo, Path: req.Path,
		Start: req.Start, End: req.End, Side: req.Side, Source: "local", CreatedAt: now,
		Comments: []reviewComment{{Author: author, Body: req.Body, CreatedAt: now}},
	}
	threads := append(s.loadThreads(req.Branch), t)
	if err := s.saveThreads(req.Branch, threads); err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	return map[string]any{"ok": true, "id": t.ID}
}

func (s *Server) ReviewThreadReply(branch, id, body, author string, isMain bool) map[string]any {
	if strings.TrimSpace(body) == "" {
		return map[string]any{"ok": false, "error": "empty comment"}
	}
	threads := s.loadThreads(branch)
	if author == "" {
		author = "you"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	found := false
	for i := range threads {
		if threads[i].ID == id {
			threads[i].Comments = append(threads[i].Comments, reviewComment{Author: author, Body: body, CreatedAt: now})
			found = true
			break
		}
	}
	if !found {
		return map[string]any{"ok": false, "error": "thread not found"}
	}
	if err := s.saveThreads(branch, threads); err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	return map[string]any{"ok": true}
}

func (s *Server) ReviewThreadResolve(branch, id string, resolved, isMain bool) map[string]any {
	threads := s.loadThreads(branch)
	found := false
	for i := range threads {
		if threads[i].ID == id {
			threads[i].Resolved = resolved
			found = true
			break
		}
	}
	if !found {
		return map[string]any{"ok": false, "error": "thread not found"}
	}
	if err := s.saveThreads(branch, threads); err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	return map[string]any{"ok": true}
}
