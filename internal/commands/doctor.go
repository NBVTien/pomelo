package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/pomelohq/pomelo/internal/config"
	"github.com/pomelohq/pomelo/internal/paths"
)

func Doctor() error {
	fmt.Printf("%sPomelo doctor%s\n\n", Bold, NC)
	critical := 0

	ok := func(label string) { fmt.Printf("  %s✓%s %s\n", Green, NC, label) }
	warn := func(label, hint string) { fmt.Printf("  %s•%s %s — %s\n", Yellow, NC, label, hint) }
	bad := func(label, hint string) { fmt.Printf("  %s✗%s %s — %s\n", "\033[31m", NC, label, hint); critical++ }

	fmt.Println("Required tools:")
	for _, d := range []struct{ name, hint string }{
		{"git", "brew install git  (or apt install git)"},
		{"zsh", "brew install zsh  (services launch via zsh)"},
	} {
		if have(d.name) {
			ok(d.name)
		} else {
			bad(d.name, d.hint)
		}
	}

	fmt.Println("\nOptional tools:")
	if have("docker") {
		if run("docker", "info") {
			ok("docker running")
		} else {
			warn("docker installed but not running", "start Docker (shared services stay off until then)")
		}
	} else {
		warn("docker", "install to use shared services (Postgres/Redis/…)")
	}
	if have("claude") {
		ok("claude CLI")
	} else {
		warn("claude", "install Claude Code for agent tabs")
	}

	fmt.Println("\nProject config:")
	if cfgPath, err := config.FindConfig(); err != nil {
		warn("pom.yml", "none in scope — run `pom init` in a project, or cd into one")
	} else if cfg, err := config.Load(cfgPath); err != nil {
		bad("config loads", err.Error())
	} else if err := cfg.Validate(); err != nil {
		bad("config valid", err.Error())
	} else {
		ok(fmt.Sprintf("config valid (%s)", cfgPath))
	}

	fmt.Println("\nState:")
	dir := paths.EnsureStateDir()
	probe := dir + "/.doctor-write-test"
	if os.WriteFile(probe, []byte("x"), 0o600) == nil {
		_ = os.Remove(probe)
		ok("state dir writable (" + dir + ")")
	} else {
		bad("state dir writable", "check permissions on "+dir)
	}

	fmt.Println()
	if critical > 0 {
		return fmt.Errorf("%d critical issue(s) — fix the ✗ above", critical)
	}
	fmt.Printf("%sAll good.%s\n", Green, NC)
	return nil
}

func have(bin string) bool { _, err := exec.LookPath(bin); return err == nil }

func run(name string, args ...string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Run() == nil
}
