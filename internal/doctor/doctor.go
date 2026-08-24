package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pomelohq/pomelo/internal/config"
	"github.com/pomelohq/pomelo/internal/secrets"
	"github.com/pomelohq/pomelo/internal/services"
)

type Severity string

const (
	SevError Severity = "error"
	SevWarn  Severity = "warn"
	SevOK    Severity = "ok"
)

type Finding struct {
	ID           string   `json:"id"`
	Severity     Severity `json:"severity"`
	Title        string   `json:"title"`
	Detail       string   `json:"detail,omitempty"`
	Fix          string   `json:"fix,omitempty"`
	AgentFixable bool     `json:"agent_fixable"`
}

var secretRef = regexp.MustCompile(`\{\{\s*secret\.([A-Za-z0-9_]+)\s*\}\}`)

func Diagnose(cfg *config.Config, projectRoot, session string) []Finding {
	var out []Finding
	add := func(f Finding) { out = append(out, f) }

	if cfg == nil {
		add(Finding{ID: "config.load", Severity: SevError, Title: "No project config",
			Detail: "pom.yml not found or failed to load", Fix: "run `pom init` or open a project folder"})
		return out
	}
	if err := cfg.Validate(); err != nil {
		add(Finding{ID: "config.validate", Severity: SevError, Title: "Config is invalid",
			Detail: err.Error(), Fix: "fix the reported key (typo'd alias / unknown {{var}})", AgentFixable: true})
	}

	if !have("git") {
		add(Finding{ID: "tool.git", Severity: SevError, Title: "git not found", Fix: "install git"})
	}
	if !have("docker") {
		if len(cfg.SharedServices) > 0 {
			add(Finding{ID: "tool.docker", Severity: SevError, Title: "docker not found",
				Detail: "shared services (postgres/redis/…) need Docker", Fix: "install Docker Desktop / OrbStack"})
		}
	} else if len(cfg.SharedServices) > 0 && !run("docker", "info") {
		add(Finding{ID: "docker.down", Severity: SevError, Title: "Docker not running",
			Detail: "shared services can't start until Docker is up", Fix: "start Docker"})
	}

	def := cfg.GlobalDefaultBranch()
	for _, name := range cfg.RepoOrder {
		if cfg.Repos[name] == nil {
			continue
		}
		p := services.RepoWorktreePath(projectRoot, name, def, true)
		if !dirExists(p) {
			add(Finding{ID: "repo.missing:" + name, Severity: SevError,
				Title: "Repo not found: " + name, Detail: p,
				Fix: "clone/prepare it (run `pom prepare-main`), or fix the repo path in config"})
		}
	}

	if rk := config.RemovedKeys(filepath.Join(projectRoot, "pom.yml")); len(rk) > 0 {
		add(Finding{ID: "config.removed", Severity: SevError, AgentFixable: true,
			Title:  "Removed config keys: " + strings.Join(rk, ", "),
			Detail: "these keys were removed from the schema and are ignored",
			Fix:    "run config_normalize (or delete them)"})
	}

	merged := ""
	if yaml, _, err := config.MergedYAML(projectRoot + "/pom.yml"); err == nil {
		merged = string(yaml)
	}

	set := map[string]bool{}
	for _, n := range secrets.Names(session) {
		set[n] = true
	}
	if merged != "" {
		missing := map[string]bool{}
		for _, m := range secretRef.FindAllStringSubmatch(merged, -1) {
			if !set[m[1]] {
				missing[m[1]] = true
			}
		}
		names := make([]string, 0, len(missing))
		for n := range missing {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			add(Finding{ID: "secret.missing:" + n, Severity: SevWarn,
				Title:  "Secret not set: " + n,
				Detail: "config uses {{secret." + n + "}} but no value is stored",
				Fix:    "add it in Settings › Integrations › Secrets"})
		}
	}

	if merged != "" {
		names := make([]string, 0, len(cfg.SharedServices))
		for n := range cfg.SharedServices {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			re := regexp.MustCompile(`\{\{\s*shared\.` + regexp.QuoteMeta(n) + `\.`)
			if !re.MatchString(merged) {
				add(Finding{ID: "shared.unwired:" + n, Severity: SevWarn, AgentFixable: true,
					Title: "Shared service not wired: " + n,
					Detail: "shared_services." + n + " is declared but no repo env references {{shared." + n +
						".url}} / {{shared." + n + ".host}} / {{shared." + n + ".port}} — its services will start with no connection info",
					Fix: "add the connection env to the repos that need it (e.g. DATABASE_URL: postgresql://{{shared." + n + ".url}}/{{db.main}})"})
			}
		}
	}

	if len(out) == 0 {
		add(Finding{ID: "ok", Severity: SevOK, Title: "Ready to run", Detail: "no blocking gaps found"})
	}
	return out
}

func have(tool string) bool { _, err := exec.LookPath(tool); return err == nil }

func run(name string, args ...string) bool {
	return exec.Command(name, args...).Run() == nil
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
