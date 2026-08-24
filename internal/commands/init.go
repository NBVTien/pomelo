package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pomelohq/pomelo/internal/sessions"
)

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func Init(name string, useClaude bool) error {
	top, err := gitTop(".")
	if err != nil {
		return fmt.Errorf("not a git repo — run `pom init` inside your project's git repo")
	}
	repoName := filepath.Base(top)
	if name == "" {
		name = repoName
	}
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid project name %q", name)
	}

	projectDir := filepath.Join(sessions.SessionsRoot(), name)
	if _, err := os.Stat(projectDir); err == nil {
		return fmt.Errorf("a project already exists at %s", projectDir)
	}
	dest := filepath.Join(projectDir, "workspace--main", repoName)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	fmt.Printf("%s>>>%s cloning %s → %s\n", Blue, NC, repoName, dest)
	if out, err := exec.Command("git", "clone", "--local", "--quiet", top, dest).CombinedOutput(); err != nil {
		_ = os.RemoveAll(projectDir)
		return fmt.Errorf("clone failed: %v\n%s", err, out)
	}
	if url := gitOriginURL(top); url != "" {
		_ = exec.Command("git", "-C", dest, "remote", "set-url", "origin", url).Run()
	}
	carryUncommitted(top, dest)

	svcName, cmd, port := detectService(top)
	if err := os.WriteFile(filepath.Join(projectDir, "pom.yml"), []byte(initConfig(name, repoName, svcName, cmd, port)), 0o644); err != nil {
		_ = os.RemoveAll(projectDir)
		return err
	}

	reg := sessions.Load()
	reg.Touch(name, projectDir, time.Now().Unix())
	_ = reg.Save()

	fmt.Printf("\n%sProject created%s at %s\n", Green, NC, projectDir)
	fmt.Printf("  service %q detected: %s\n", svcName, cmd)

	if useClaude {
		return refineWithClaude(projectDir, repoName)
	}
	fmt.Printf("  next:  cd %s && pom\n", projectDir)
	fmt.Printf("  note:  gitignored files (e.g. .env) aren't cloned — copy them into\n         %s if your app needs them\n", dest)
	return nil
}

func refineWithClaude(projectDir, repoName string) error {
	if _, err := exec.LookPath("claude"); err != nil {
		fmt.Printf("  (claude not found — starter pom.yml is ready; `cd %s && pom`)\n", projectDir)
		return nil
	}
	fmt.Printf("\n%s>>>%s handing off to claude to tailor pom.yml — answer its questions\n\n", Blue, NC)
	c := exec.Command("claude", claudeInitPrompt(repoName))
	c.Dir = projectDir
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Run()
}

func claudeInitPrompt(repo string) string {
	return fmt.Sprintf(`You are helping set up a Pomelo (pom) dev-environment config. The project's `+
		`repo is in ./workspace--main/%s and a minimal starter ./pom.yml already exists.

Improve ./pom.yml for this project — but this is a CONVERSATION: ASK the user, don't assume, confirm each decision. Do NOT do everything silently.

Steps:
1. Read ./workspace--main/%s (package.json, Gemfile, go.mod, docker-compose*.yml, .env.example, README) to understand its services, ports, databases and env.
2. Ask the user ONE topic at a time — which services to run (name + command), which listen on a port, which databases each needs, which shared services (postgres/redis/minio/opensearch), and key env vars. Propose defaults from what you found, then WAIT for the answer.
3. Edit ./pom.yml to match. Keep it minimal and working.

pom.yml essentials:
- session, default_branch, repos:{%s:{services, env, databases}}
- a service: { cmd: "...", port: true }  (port:true assigns $PORT; reference another service via {{<repo>.<svc>.url}})
- databases: { main: "{{branch.safe}}" }  (per-branch DB name)
- env values use dot-notation templates: {{shared.postgres.url}}, {{db.main}}, {{<repo>.<svc>.url}}, {{shared.NAME.host}}, {{shared.NAME.port}}, {{branch}}, {{branch.safe}}, {{branch.host}}
- shared_services: just NAME them (postgres/redis/minio/opensearch) — Pomelo fills image/ports/creds by default.

Start by summarizing what you found in the repo, then ask your first question.`, repo, repo, repo)
}

func gitTop(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitOriginURL(dir string) string {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func carryUncommitted(src, dest string) {
	out, err := exec.Command("git", "-C", src, "ls-files", "-m", "-o", "--exclude-standard").Output()
	if err != nil {
		return
	}
	for _, rel := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if rel == "" {
			continue
		}
		s := filepath.Join(src, rel)
		if st, err := os.Stat(s); err != nil || st.IsDir() {
			continue
		}
		d := filepath.Join(dest, rel)
		_ = os.MkdirAll(filepath.Dir(d), 0o755)
		_ = copyFile(s, d, 0o644)
	}
}

func detectService(dir string) (name, cmd string, port bool) {
	has := func(f string) bool { _, err := os.Stat(filepath.Join(dir, f)); return err == nil }
	switch {
	case has("package.json"):
		pm := "npm"
		switch {
		case has("pnpm-lock.yaml"):
			pm = "pnpm"
		case has("yarn.lock"):
			pm = "yarn"
		case has("bun.lockb"):
			pm = "bun"
		}
		script := "dev"
		if s := pickNpmScript(filepath.Join(dir, "package.json")); s != "" {
			script = s
		}
		return "dev", pm + " run " + script, true
	case has("Gemfile"):
		if has("bin/rails") {
			return "web", "bundle exec rails s -p $PORT", true
		}
		return "web", "bundle exec rackup -p $PORT", true
	case has("manage.py"):
		return "web", "python manage.py runserver 0.0.0.0:$PORT", true
	case has("go.mod"):
		return "app", "go run .", false
	case has("Cargo.toml"):
		return "app", "cargo run", false
	default:
		return "dev", "echo 'set your dev command in pom.yml' && sleep infinity", false
	}
}

func pickNpmScript(pkgPath string) string {
	b, err := os.ReadFile(pkgPath)
	if err != nil {
		return ""
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(b, &pkg) != nil {
		return ""
	}
	for _, s := range []string{"dev", "start", "serve"} {
		if _, ok := pkg.Scripts[s]; ok {
			return s
		}
	}
	return ""
}

func initConfig(session, repo, svc, cmd string, port bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "session: %s\n", session)
	b.WriteString("default_branch: main\n")
	b.WriteString("repos:\n")
	fmt.Fprintf(&b, "  %s:\n", repo)
	b.WriteString("    services:\n")
	fmt.Fprintf(&b, "      %s:\n", svc)
	fmt.Fprintf(&b, "        cmd: %q\n", cmd)
	if port {
		b.WriteString("        port: true\n")
	}
	return b.String()
}
