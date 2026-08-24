package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommands_DeriveEffective(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pom.yml")
	os.WriteFile(path, []byte(
		"session: t\nrepos:\n  api:\n    commands:\n      install: npm ci\n      generate: npm run gen\n      migrate: npm run migrate\n      test: npm test\n"), 0o644)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	d := cfg.Repos["api"]

	if got := d.EffectiveSetup(); len(got) != 3 || got[0] != "npm ci" || got[1] != "npm run gen" || got[2] != "npm run migrate" {
		t.Errorf("EffectiveSetup = %v, want [npm ci, npm run gen, npm run migrate]", got)
	}
	if got := d.EffectiveMigrate(); len(got) != 1 || got[0] != "npm run migrate" {
		t.Errorf("EffectiveMigrate = %v, want [npm run migrate]", got)
	}
	scs := d.EffectiveShortcuts()
	if len(scs) != 4 {
		t.Fatalf("EffectiveShortcuts len = %d, want 4 (%v)", len(scs), scs)
	}
	want := []string{"npm ci", "npm run gen", "npm run migrate", "npm test"}
	for i, w := range want {
		if scs[i].Cmd != w {
			t.Errorf("shortcut[%d].Cmd = %q, want %q", i, scs[i].Cmd, w)
		}
	}
}

func TestCommands_ExplicitWins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pom.yml")
	os.WriteFile(path, []byte(
		"session: t\nrepos:\n  api:\n    setup: [explicit setup]\n    migrate: [explicit migrate]\n    commands:\n      install: npm ci\n      migrate: npm run migrate\n"), 0o644)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	d := cfg.Repos["api"]
	if got := d.EffectiveSetup(); len(got) != 1 || got[0] != "explicit setup" {
		t.Errorf("EffectiveSetup = %v, want [explicit setup]", got)
	}
	if got := d.EffectiveMigrate(); len(got) != 1 || got[0] != "explicit migrate" {
		t.Errorf("EffectiveMigrate = %v, want [explicit migrate]", got)
	}
}

func TestPreset_Composition(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pom.yml")
	os.WriteFile(path, []byte(
		"session: t\n"+
			"presets:\n"+
			"  infra:\n    env:\n      REDIS_URL: redis://x\n"+
			"  pg:\n    env:\n      DATABASE_URL: postgres://y\n"+
			"  nest:\n    preset: [infra, pg]\n    seed_from_main: true\n    pre_start: source .env.local\n"+
			"    commands:\n      install: npm ci\n"+
			"    services:\n      worker: node worker\n"+
			"repos:\n  ai:\n    preset: nest\n    services:\n      api:\n        port: true\n        cmd: node main\n"), 0o644)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	d := cfg.Repos["ai"]
	if d.Env["REDIS_URL"] != "redis://x" || d.Env["DATABASE_URL"] != "postgres://y" {
		t.Errorf("composed env not folded: %v", d.Env)
	}
	if !d.SeedFromMain {
		t.Error("seed_from_main from preset not applied")
	}
	if d.PreStart != "source .env.local" {
		t.Errorf("pre_start from preset = %q", d.PreStart)
	}
	if d.Commands["install"] != "npm ci" {
		t.Errorf("commands from composed preset = %v", d.Commands)
	}
	if _, ok := d.Services["worker"]; !ok {
		t.Errorf("worker service from preset missing: %v", d.ServiceOrder)
	}
	if _, ok := d.Services["api"]; !ok {
		t.Error("repo's own api service dropped")
	}
}

func TestPreset_Composition_RepoWins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pom.yml")
	os.WriteFile(path, []byte(
		"session: t\n"+
			"presets:\n"+
			"  base:\n    pre_start: base start\n"+
			"  nest:\n    preset: [base]\n    pre_start: nest start\n"+
			"repos:\n  a:\n    preset: nest\n    lifecycle:\n      pre_start: repo start\n"), 0o644)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Repos["a"].PreStart; got != "repo start" {
		t.Errorf("pre_start = %q, want 'repo start' (repo wins)", got)
	}
}

func TestCommands_FromPreset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pom.yml")
	os.WriteFile(path, []byte(
		"session: t\npresets:\n  node:\n    commands:\n      install: npm ci\n      migrate: npm run migrate\nrepos:\n  api:\n    preset: [node]\n"), 0o644)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	d := cfg.Repos["api"]
	if got := d.EffectiveMigrate(); len(got) != 1 || got[0] != "npm run migrate" {
		t.Errorf("EffectiveMigrate = %v, want [npm run migrate] (from preset)", got)
	}
	if got := d.Commands["install"]; got != "npm ci" {
		t.Errorf("Commands[install] = %q, want npm ci (from preset)", got)
	}
}
