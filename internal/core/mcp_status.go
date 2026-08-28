package core

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pomelohq/pomelo/internal/agent/claude"
)

func (s *Server) MCPGlobalStatus() map[string]any {
	home, _ := os.UserHomeDir()
	out := map[string]any{"registered": false, "connected": false, "command": "", "wrapper_ok": false, "list_line": ""}

	script := filepath.Join(home, ".local", "state", "pom", "pom-mcp")
	if fi, err := os.Stat(script); err == nil && fi.Mode()&0o111 != 0 {
		out["wrapper_ok"] = true
	}
	out["wrapper"] = script

	if b, err := os.ReadFile(filepath.Join(home, ".claude.json")); err == nil {
		var root map[string]any
		if json.Unmarshal(b, &root) == nil {
			if servers, ok := root["mcpServers"].(map[string]any); ok {
				if pom, ok := servers["pom"].(map[string]any); ok {
					out["registered"] = true
					cmd, _ := pom["command"].(string)
					args, _ := pom["args"].([]any)
					parts := []string{cmd}
					for _, a := range args {
						if s, ok := a.(string); ok {
							parts = append(parts, s)
						}
					}
					out["command"] = strings.TrimSpace(strings.Join(parts, " "))
				}
			}
		}
	}

	if b, ok := out["wrapper_ok"].(bool); ok && b {
		probe := exec.Command("sh", script)
		probe.Stdin = strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"pomelo","version":"1"}}}` + "\n")
		done := make(chan struct{})
		var probeOut []byte
		go func() { probeOut, _ = probe.CombinedOutput(); close(done) }()
		select {
		case <-done:
		case <-time.After(6 * time.Second):
			if probe.Process != nil {
				_ = probe.Process.Kill()
			}
		}
		s := string(probeOut)
		out["connected"] = strings.Contains(s, `"result"`) && strings.Contains(s, "serverInfo")
	}
	inst, current, skillPath := claude.SkillsInstalled()
	out["skill_installed"] = inst
	out["skill_current"] = current
	out["skill_path"] = skillPath
	return out
}

func (s *Server) MCPGlobalReinstall() map[string]any {
	if err := claude.InstallGlobalClaudeMCP(); err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	if err := claude.InstallGlobalSkills(); err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	return map[string]any{"ok": true}
}
