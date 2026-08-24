package services

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pomelohq/pomelo/internal/config"
	"github.com/pomelohq/pomelo/internal/tmpl"
)

var ticketLabelRe = regexp.MustCompile(`^[a-z]+-[0-9]+`)

func WorkspaceLabel(branch string) string {
	h := BranchHost(branch)
	if m := ticketLabelRe.FindString(h); m != "" {
		return m
	}
	return h
}

type WorktreeInfo struct {
	Branch    string
	ParentDir string
	Path      string
}

func BranchSafe(branch string) string {
	return strings.ReplaceAll(branch, "/", "_")
}

func PortWsKey(branch string) string {
	return "ws-" + BranchSafe(branch)
}

func BranchHash(branch string) string {
	sum := sha1.Sum([]byte(branch))
	return hex.EncodeToString(sum[:])[:8]
}

func BranchHost(branch string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(branch) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('-')
		}
	}
	label := strings.Trim(sb.String(), "-")
	if len(label) <= dnsLabelMax {
		return label
	}
	sum := sha1.Sum([]byte(branch))
	suffix := hex.EncodeToString(sum[:])[:8]
	prefix := strings.Trim(label[:dnsLabelMax-len(suffix)-1], "-")
	return prefix + "-" + suffix
}

const dnsLabelMax = 63

func ResolveBranchTokens(val, branch string) string {
	if !strings.Contains(val, "{{branch") {
		return val
	}
	safe, hash, host := BranchSafe(branch), BranchHash(branch), BranchHost(branch)
	r := strings.NewReplacer(
		"{{branch.safe}}", safe, "{{branch|safe}}", safe, "{{branch_safe}}", safe,
		"{{branch.hash}}", hash, "{{branch|hash}}", hash, "{{branch_hash}}", hash,
		"{{branch.host}}", host, "{{branch|host}}", host, "{{branch_host}}", host,
		"{{branch}}", branch,
	)
	return r.Replace(val)
}

func SlotRefsIn(s string) []string {
	var out []string
	for _, k := range tmpl.Refs(s) {
		if strings.HasPrefix(k, "shared.") && strings.HasSuffix(k, ".slot") {
			out = append(out, strings.TrimSuffix(strings.TrimPrefix(k, "shared."), ".slot"))
		}
	}
	return out
}

const DefaultProxyPort = 8767

func proxyPort(*config.Config) int { return DefaultProxyPort }

func ResolveEnvTemplates(env map[string]string, cfg *config.Config, branchSafe, branch, wsKey, envName string, dbNames map[string]string) []EnvVar {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ctx := ResolveCtx{Cfg: cfg, Branch: branch, WsKey: wsKey, EnvName: envName, DBNames: dbNames}
	result := make([]EnvVar, 0, len(env))
	for _, k := range keys {
		result = append(result, EnvVar{Key: k, Value: ResolveTokens(env[k], ctx)})
	}
	return result
}

func findRepoServicePort(cfg *config.Config, name, wsKey string) int {
	repoName, svcName, _ := strings.Cut(name, "/")

	for dirName, dir := range cfg.Repos {
		if dirName != repoName && dir.Alias != repoName {
			continue
		}
		alias := dir.Alias
		if alias == "" {
			alias = dirName
		}

		if svcName != "" {
			svcKey := alias + "~" + svcName
			if p := Port(currentProjectDir, wsKey, svcKey); p > 0 {
				return p
			}
			return 0
		}

		if len(dir.ServiceOrder) > 0 {
			svcKey := alias + "~" + dir.ServiceOrder[0]
			if p := Port(currentProjectDir, wsKey, svcKey); p > 0 {
				return p
			}
		}
		if dir.ProxyPort != nil {
			return int(*dir.ProxyPort)
		}
		return 0
	}
	return 0
}

func ExtractPortFromCmd(cmd string) uint16 {
	parts := strings.Fields(cmd)
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "--port" || parts[i] == "-p" {
			var p uint16
			if _, err := fmt.Sscanf(parts[i+1], "%d", &p); err == nil {
				return p
			}
		}
	}
	for _, part := range parts {
		if val, ok := strings.CutPrefix(part, "--port="); ok {
			var p uint16
			if _, err := fmt.Sscanf(val, "%d", &p); err == nil {
				return p
			}
		}
	}
	return 0
}

func WorkspaceFolderPath(configDir, name string) string {
	return filepath.Join(configDir, "workspace--"+name)
}

func WorkspaceRootDir(projectRoot, branch string, isMain bool) string {
	dir := filepath.Join(projectRoot, "workspace--"+branch)
	if isMain && !DirExists(dir) {
		return projectRoot
	}
	return dir
}

func RepoWorktreePath(projectRoot, repo, branch string, isMain bool) string {
	return filepath.Join(WorkspaceRootDir(projectRoot, branch, isMain), repo)
}

func httpToWs(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	return u.String()
}
