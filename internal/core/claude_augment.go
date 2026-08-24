package core

import (
	"encoding/json"
	"fmt"
	"github.com/pomelohq/pomelo/internal/provider/shell"
	"os"
	"path/filepath"
	"strings"

	"github.com/pomelohq/pomelo/internal/agent/codeagent"
	"github.com/pomelohq/pomelo/internal/jira"
	"github.com/pomelohq/pomelo/internal/pombin"
	"github.com/pomelohq/pomelo/internal/services"
)

func (s *Server) startWsWindow(holder, cmd string, ref serviceRef) error {
	if s.WorkspaceRoot == "" {
		return fmt.Errorf("workspace root unknown")
	}
	cwd := s.workspaceRoot(ref.Branch, ref.IsMain)
	shellCmd := s.claudeAugment(cmd, ref)
	return services.SpawnHolder(holder, cwd, 0, 0, shell.Login(shellCmd))
}

func (s *Server) claudeAugment(cmd string, ref serviceRef) string {
	if s.cfg() == nil || codeagent.LookupAgent(ref.Svc) == nil {
		return cmd
	}
	fields := strings.Fields(cmd)
	if len(fields) == 0 || filepath.Base(fields[0]) != "claude" {
		return cmd
	}
	hint := s.ticketContext(ref.Branch) +
		"You have the `pom` MCP tools to inspect and act on THIS workspace's real environment: " +
		"services, ports, databases (connection strings), and resolve_port_conflict. " +
		"Before hand-writing any shell command, call `commands` — the project defines pre-written `setup` " +
		"steps and `shortcuts` (install/generate/migrate/lint/test/build) plus its package manager (local_pm, " +
		"e.g. pnpm); run them with `run_shortcut` (by description) so you use the project's canonical command, " +
		"and fall back to `run_in_env` only when none fits. Use the project's package manager, not npm/yarn. " +
		"For config, use config_get/config_validate to read+check, and to edit: if the config is split " +
		"(config_files shows pom.d/** fragments) edit the right fragment with config_file_get/config_file_set; " +
		"otherwise use config_set. Every write is validated against the full merged config before it lands. " +
		"Prefer these tools over guessing ports, commands, or hand-editing files."
	rest := strings.TrimSpace(strings.TrimPrefix(cmd, fields[0]))
	out := fields[0] +
		" --append-system-prompt " + shell.Quote(hint)
	if rest != "" {
		out += " " + rest
	}
	return out
}

func (s *Server) ticketContext(branch string) string {
	key := jira.KeyForBranch(branch)
	if key == "" {
		return ""
	}
	jc := jira.Resolve(s.cfg())
	if jc == nil {
		return ""
	}
	iss, err := jc.IssueWithDescription(key)
	if err != nil || iss.Summary == "" {
		return ""
	}
	b := "You are working on ticket " + iss.Key + ": " + iss.Summary + ".\n"
	if d := strings.TrimSpace(iss.Description); d != "" {
		if len(d) > 4000 {
			d = d[:4000] + "…"
		}
		b += "\nTicket description:\n" + d + "\n"
	}
	return b + "\n"
}

func (s *Server) mcpConfigJSON(branch string) string {
	self, err := pombin.Path()
	if err != nil || self == "" {
		self = "pom"
	}
	if strings.Contains(self, " ") {
		if home, e := os.UserHomeDir(); e == nil {
			dir := filepath.Join(home, ".local", "state", "pom")
			link := filepath.Join(dir, "pom-mcp-bin")
			_ = os.MkdirAll(dir, 0o755)
			if cur, _ := os.Readlink(link); cur != self {
				_ = os.Remove(link)
				_ = os.Symlink(self, link)
			}
			if fi, e2 := os.Lstat(link); e2 == nil && fi.Mode()&os.ModeSymlink != 0 {
				self = link
			}
		}
	}
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"pom": map[string]any{
				"command": self,
				"args":    []string{"mcp", "--branch", branch},
			},
		},
	}
	b, _ := json.Marshal(cfg)
	return string(b)
}
