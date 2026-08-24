package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pomelohq/pomelo/internal/secrets"
	"github.com/pomelohq/pomelo/internal/sessions"
	"gopkg.in/yaml.v3"
)

type RepoSpec struct {
	Path  string `json:"path"`
	Alias string `json:"alias,omitempty"`
}

type EnvFileSpec struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type CreateSessionReq struct {
	Name          string        `json:"name"`
	Root          string        `json:"root,omitempty"`
	DefaultBranch string        `json:"default_branch,omitempty"`
	Repos         []RepoSpec    `json:"repos"`
	EnvFiles      []EnvFileSpec `json:"env_files,omitempty"`
}

func (s *Server) handleSessionCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req CreateSessionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	sessionDir, err := ScaffoldSession(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "name": strings.TrimSpace(req.Name), "path": sessionDir})
}

func ScaffoldSession(req CreateSessionReq) (string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid session name")
	}
	root := strings.TrimSpace(req.Root)
	if root == "" {
		root = sessions.SessionsRoot()
	}
	defaultBranch := strings.TrimSpace(req.DefaultBranch)
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	sessionDir := filepath.Join(root, name)
	if _, err := os.Stat(sessionDir); err == nil {
		return "", fmt.Errorf("a session directory already exists at %s", sessionDir)
	}
	wsDir := filepath.Join(sessionDir, "workspace--"+defaultBranch)
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return "", err
	}

	reposCfg := map[string]map[string]any{}
	sharedFromCompose := map[string]any{}
	var needPostgres, needRedis bool
	for _, repo := range req.Repos {
		src := strings.TrimSpace(repo.Path)
		if src == "" {
			continue
		}
		if strings.HasPrefix(src, "-") {
			_ = os.RemoveAll(sessionDir)
			return "", fmt.Errorf("invalid repo source: %s", src)
		}

		var repoName string
		var cloneErr error
		if isGitURL(src) {
			repoName = repoNameFromURL(src)
			if repoName == "" {
				_ = os.RemoveAll(sessionDir)
				return "", fmt.Errorf("cannot derive repo name from URL: %s", src)
			}
			cloneErr = cloneRemote(src, filepath.Join(wsDir, repoName))
		} else {
			if !isGitRepo(src) {
				_ = os.RemoveAll(sessionDir)
				return "", fmt.Errorf("not a git repo: %s", src)
			}
			repoName = filepath.Base(strings.TrimRight(src, "/"))
			cloneErr = cloneRepoWithChanges(src, filepath.Join(wsDir, repoName))
			importIgnoredEnv(src, name)
		}
		if cloneErr != nil {
			_ = os.RemoveAll(sessionDir)
			return "", fmt.Errorf("clone %s: %w", repoName, cloneErr)
		}
		repoPath := filepath.Join(wsDir, repoName)
		entry, pg, redis := detectStack(repoPath)
		if repo.Alias != "" {
			entry["alias"] = repo.Alias
		}
		needPostgres = needPostgres || pg
		needRedis = needRedis || redis
		for k, v := range detectSharedFromCompose(repoPath) {
			sharedFromCompose[k] = v
		}
		reposCfg[repoName] = entry
	}

	for _, ef := range req.EnvFiles {
		rel := strings.TrimSpace(ef.Path)
		if rel == "" {
			continue
		}
		if filepath.IsAbs(rel) || strings.Contains(rel, "..") {
			continue
		}
		dst := filepath.Join(wsDir, filepath.Clean(rel))
		if dst != wsDir && !strings.HasPrefix(dst, wsDir+string(os.PathSeparator)) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			continue
		}
		_ = os.WriteFile(dst, []byte(ef.Content), 0o644)
	}

	doc := map[string]any{
		"session":        name,
		"default_branch": defaultBranch,
		"repos":          reposCfg,
	}
	shared := map[string]any{}
	for k, v := range sharedFromCompose {
		shared[k] = v
	}
	if needPostgres {
		if _, ok := shared["postgres"]; !ok {
			shared["postgres"] = map[string]any{}
		}
	}
	if needRedis {
		if _, ok := shared["redis"]; !ok {
			shared["redis"] = map[string]any{}
		}
	}
	if len(shared) > 0 {
		doc["shared_services"] = shared
	}
	yml, err := yaml.Marshal(doc)
	if err != nil {
		_ = os.RemoveAll(sessionDir)
		return "", err
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "pom.yml"), yml, 0o644); err != nil {
		_ = os.RemoveAll(sessionDir)
		return "", err
	}

	reg := sessions.Load()
	reg.Touch(name, sessionDir, time.Now().Unix())
	_ = reg.Save()
	return sessionDir, nil
}

func importIgnoredEnv(srcRepo, session string) {
	out, err := exec.Command("git", "-C", srcRepo, "ls-files", "--others", "--ignored", "--exclude-standard", "-z").Output()
	if err != nil {
		return
	}
	for _, rel := range strings.Split(string(out), "\x00") {
		base := filepath.Base(rel)
		if rel == "" || !strings.HasPrefix(base, ".env") {
			continue
		}
		if strings.Contains(base, ".example") || strings.Contains(base, ".sample") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(srcRepo, rel))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			k, v, ok := parseEnvLine(line)
			if ok {
				_ = secrets.Set(session, k, v)
			}
		}
	}
}

func parseEnvLine(line string) (key, val string, ok bool) {
	s := strings.TrimSpace(line)
	if s == "" || strings.HasPrefix(s, "#") {
		return "", "", false
	}
	s = strings.TrimPrefix(s, "export ")
	i := strings.IndexByte(s, '=')
	if i <= 0 {
		return "", "", false
	}
	key = strings.TrimSpace(s[:i])
	val = strings.TrimSpace(s[i+1:])
	if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
		val = val[1 : len(val)-1]
	}
	if key == "" {
		return "", "", false
	}
	return key, val, true
}

func isGitURL(s string) bool {
	if strings.Contains(s, "://") {
		return true
	}
	if i := strings.IndexByte(s, ':'); i > 0 && !strings.HasPrefix(s, "/") && !strings.HasPrefix(s, ".") {
		return true
	}
	return false
}

func repoNameFromURL(u string) string {
	u = strings.TrimSuffix(strings.TrimRight(u, "/"), ".git")
	if i := strings.LastIndexAny(u, "/:"); i >= 0 {
		u = u[i+1:]
	}
	return strings.TrimSuffix(u, ".git")
}

func cloneRemote(url, dst string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", "--", url, dst)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func cloneRepoWithChanges(src, dst string) error {
	if out, err := exec.Command("git", "clone", "--local", src, dst).CombinedOutput(); err != nil {
		return fmt.Errorf("clone: %s", strings.TrimSpace(string(out)))
	}
	if origin, err := exec.Command("git", "-C", src, "remote", "get-url", "origin").Output(); err == nil {
		if u := strings.TrimSpace(string(origin)); u != "" {
			_ = exec.Command("git", "-C", dst, "remote", "set-url", "origin", u).Run()
		}
	}
	out, err := exec.Command("git", "-C", src, "ls-files", "-m", "-o", "--exclude-standard", "-z").Output()
	if err != nil {
		return nil
	}
	for _, rel := range strings.Split(string(out), "\x00") {
		if rel == "" {
			continue
		}
		_ = copyFileInto(filepath.Join(src, rel), filepath.Join(dst, rel))
	}
	return nil
}

func copyFileInto(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func detectStack(repoDir string) (entry map[string]any, needPostgres, needRedis bool) {
	entry = map[string]any{}
	pm := detectPM(repoDir)
	raw, err := os.ReadFile(filepath.Join(repoDir, "package.json"))
	if err != nil {
		return entry, false, false
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Scripts         map[string]string `json:"scripts"`
		Workspaces      json.RawMessage   `json:"workspaces"`
	}
	_ = json.Unmarshal(raw, &pkg)
	dep := func(name string) bool {
		_, a := pkg.Dependencies[name]
		_, b := pkg.DevDependencies[name]
		return a || b
	}
	hasScript := func(name string) bool { _, ok := pkg.Scripts[name]; return ok }

	entry["setup"] = []string{pm + " install"}

	svcName, cmd := "app", pm+" run dev"
	switch {
	case dep("@nestjs/core"):
		svcName = "api"
		if hasScript("start:dev") {
			cmd = pm + " run start:dev"
		} else {
			cmd = "npx nest start --watch"
		}
		needPostgres, needRedis = true, true
	case dep("next"):
		svcName, cmd = "web", pm+" run dev"
	case dep("vite"):
		svcName, cmd = "web", pm+" run dev"
	case dep("react-scripts"):
		svcName, cmd = "web", pm+" start"
	case hasScript("dev"):
		svcName, cmd = "app", pm+" run dev"
	case hasScript("start"):
		svcName, cmd = "app", pm+" start"
	}
	entry["services"] = map[string]any{svcName: map[string]any{"cmd": cmd}}
	return entry, needPostgres, needRedis
}

func detectPM(repoDir string) string {
	for _, c := range []struct{ file, pm string }{
		{"pnpm-lock.yaml", "pnpm"}, {"yarn.lock", "yarn"}, {"bun.lockb", "bun"}, {"package-lock.json", "npm"},
	} {
		if _, err := os.Stat(filepath.Join(repoDir, c.file)); err == nil {
			return c.pm
		}
	}
	return "npm"
}

func detectSharedFromCompose(repoDir string) map[string]any {
	out := map[string]any{}
	var raw []byte
	for _, f := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		if b, err := os.ReadFile(filepath.Join(repoDir, f)); err == nil {
			raw = b
			break
		}
	}
	if raw == nil {
		return out
	}
	var doc struct {
		Services map[string]struct {
			Image string `yaml:"image"`
		} `yaml:"services"`
	}
	if yaml.Unmarshal(raw, &doc) != nil {
		return out
	}
	known := []struct{ sub, name string }{
		{"postgres", "postgres"}, {"postgis", "postgres"}, {"redis", "redis"},
		{"minio", "minio"}, {"opensearch", "opensearch"}, {"elasticsearch", "opensearch"},
		{"mysql", "mysql"}, {"mariadb", "mysql"}, {"mongo", "mongo"},
		{"rabbitmq", "rabbitmq"}, {"kafka", "kafka"},
	}
	for _, svc := range doc.Services {
		img := strings.ToLower(svc.Image)
		if img == "" {
			continue
		}
		for _, k := range known {
			if strings.Contains(img, k.sub) {
				entry := map[string]any{}
				if _, wellKnown := map[string]bool{"postgres": true, "redis": true, "minio": true, "opensearch": true}[k.name]; !wellKnown {
					entry["image"] = svc.Image
				}
				out[k.name] = entry
				break
			}
		}
	}
	return out
}
