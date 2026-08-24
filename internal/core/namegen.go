package core

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type suggestNameReq struct {
	Branch string `json:"branch"`
	Desc   string `json:"desc"`
}

func (s *Server) handleSuggestName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpErr(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	req, err := readJSON[suggestNameReq](r)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "bad json")
		return
	}
	writeJSON(w, s.SuggestName(req.Branch, req.Desc))
}

func (s *Server) SuggestName(branch, desc string) map[string]any {
	name, slug := claudeSuggestNameSlug(branch, desc)
	return map[string]any{"name": name, "slug": slug}
}

var jsonObjRe = regexp.MustCompile(`(?s)\{.*\}`)

func claudeSuggestNameSlug(branch, desc string) (string, string) {
	var b strings.Builder
	b.WriteString("You name developer workspaces. Return COMPACT JSON only, no prose:\n")
	b.WriteString(`{"name":"<human title, Title Case, <=7 words, keep a leading ticket key UPPERCASE e.g. PROJ-1147>","slug":"<kebab-case, <=5 words, a-z0-9- only, keep the ticket key lowercase e.g. proj-1147>"}` + "\n")
	b.WriteString("Seed branch: " + branch + "\n")
	if strings.TrimSpace(desc) != "" {
		b.WriteString("Description: " + desc + "\n")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "claude", "-p",
		"--output-format", "text",
		"--disallowed-tools", "Bash Edit Write Read Glob Grep WebFetch WebSearch NotebookEdit",
	)
	cmd.Dir = os.TempDir()
	cmd.Stdin = strings.NewReader(b.String())
	out, err := cmd.Output()
	if err != nil {
		return "", ""
	}
	m := jsonObjRe.FindString(string(out))
	if m == "" {
		return "", ""
	}
	var parsed struct{ Name, Slug string }
	if json.Unmarshal([]byte(m), &parsed) != nil {
		return "", ""
	}
	name := strings.TrimSpace(parsed.Name)
	if len([]rune(name)) > 60 {
		name = strings.TrimSpace(string([]rune(name)[:60]))
	}
	return name, slugify(parsed.Slug)
}

var slugCleanRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	return strings.Trim(slugCleanRe.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-"), "-")
}
