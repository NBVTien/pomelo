package core

import (
	"github.com/pomelohq/pomelo/internal/agent/claude"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pomelohq/pomelo/internal/services"
)

const longBranchThreshold = 40

func (s *Server) workspaceNameFor(gitBranch string) string {
	if len([]rune(gitBranch)) < longBranchThreshold {
		return gitBranch
	}
	slug := ""
	if bin := claude.ResolveClaudeBin(); bin != "" {
		prompt := "Shorten this git branch name into a short kebab-case slug: lowercase words joined by hyphens, no spaces, at most 4 words. " +
			"Keep any ticket id (e.g. proj-1234) at the front. " +
			"Reply with ONLY the slug — no quotes, no explanation.\n\nBranch: " + gitBranch
		if out, err := services.RunTimeout(30*time.Second, s.WorkspaceRoot, bin, "-p", prompt); err == nil {
			slug = cleanLabel(string(out))
		}
	}
	if slug == "" {
		slug = cleanLabel(gitBranch)
		if r := []rune(slug); len(r) > longBranchThreshold {
			slug = strings.Trim(string(r[:longBranchThreshold]), "-")
		}
	}
	if slug == "" {
		return gitBranch
	}
	return s.uniqueWorkspaceName(slug)
}

func (s *Server) uniqueWorkspaceName(name string) string {
	taken := func(n string) bool {
		if n == s.DefaultBranch {
			return true
		}
		return dirExists(filepath.Join(s.WorkspaceRoot, "workspace--"+n))
	}
	if !taken(name) {
		return name
	}
	for i := 2; i < 100; i++ {
		cand := name + "-" + strconv.Itoa(i)
		if !taken(cand) {
			return cand
		}
	}
	return name
}

func cleanLabel(raw string) string {
	label := strings.TrimSpace(raw)
	if i := strings.IndexByte(label, '\n'); i >= 0 {
		label = label[:i]
	}
	label = strings.ToLower(label)
	var b strings.Builder
	prevHyphen := false
	for _, r := range label {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevHyphen = false
		default:
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 50 {
		slug = strings.Trim(slug[:50], "-")
	}
	return slug
}
