package claude

import (
	"bufio"
	"github.com/pomelohq/pomelo/internal/httpx"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type slashCommand struct {
	Name   string `json:"name"`
	Desc   string `json:"desc"`
	Source string `json:"source"`
}

var builtinCommands = []slashCommand{
	{"clear", "Start a new conversation (clear context)", "builtin"},
	{"login", "Sign in to your Claude account", "builtin"},
	{"logout", "Sign out of your Claude account", "builtin"},
	{"whoami", "Show current Claude sign-in status", "builtin"},
	{"compact", "Compact the conversation to save context", "builtin"},
	{"model", "Change the model", "builtin"},
	{"agents", "Manage agent configurations", "builtin"},
	{"add-dir", "Add a working directory", "builtin"},
	{"review", "Review a pull request", "builtin"},
	{"init", "Initialize a CLAUDE.md for the project", "builtin"},
	{"cost", "Show token usage and cost", "builtin"},
	{"config", "Open the config panel", "builtin"},
	{"mcp", "Manage MCP servers", "builtin"},
	{"memory", "Edit memory files", "builtin"},
	{"resume", "Resume a previous session", "builtin"},
	{"help", "Show help", "builtin"},
}

func (s *Feature) handleClaudeCommands(w http.ResponseWriter, r *http.Request) {
	seen := map[string]bool{}
	var cmds []slashCommand
	add := func(c slashCommand) {
		if c.Name == "" || seen[c.Name] {
			return
		}
		seen[c.Name] = true
		cmds = append(cmds, c)
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, c := range scanCommandDir(filepath.Join(home, ".claude", "commands"), "user") {
			add(c)
		}
	}
	if cwd := r.URL.Query().Get("cwd"); cwd != "" {
		for _, c := range scanCommandDir(filepath.Join(cwd, ".claude", "commands"), "project") {
			add(c)
		}
	}
	for _, c := range builtinCommands {
		add(c)
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
	httpx.Write(w, map[string]any{"commands": cmds})
}

func scanCommandDir(dir, source string) []slashCommand {
	var out []slashCommand
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		name := strings.TrimSuffix(filepath.ToSlash(rel), ".md")
		out = append(out, slashCommand{Name: name, Desc: commandDesc(path), Source: source})
		return nil
	})
	return out
}

func commandDesc(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	inFront := false
	var firstBody string
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "---" {
			inFront = !inFront
			continue
		}
		if inFront {
			if v, ok := strings.CutPrefix(line, "description:"); ok {
				return strings.TrimSpace(strings.Trim(strings.TrimSpace(v), `"'`))
			}
			continue
		}
		if line != "" && firstBody == "" {
			firstBody = line
		}
	}
	if len(firstBody) > 80 {
		firstBody = firstBody[:80]
	}
	return firstBody
}
