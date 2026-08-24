package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetUnsetRepoEnv_Flat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pom.yml")
	os.WriteFile(path, []byte("session: t\nrepos:\n  api:\n    env:\n      A: '1'\n"), 0o644)

	if err := SetRepoEnv(path, "api", map[string]string{"B": "2", "A": "9"}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := Load(path)
	if cfg.Repos["api"].Env["B"] != "2" || cfg.Repos["api"].Env["A"] != "9" {
		t.Fatalf("set failed: %v", cfg.Repos["api"].Env)
	}

	if err := UnsetRepoEnv(path, "api", []string{"A"}); err != nil {
		t.Fatal(err)
	}
	cfg, _ = Load(path)
	if _, has := cfg.Repos["api"].Env["A"]; has {
		t.Fatalf("unset failed: %v", cfg.Repos["api"].Env)
	}
	if cfg.Repos["api"].Env["B"] != "2" {
		t.Errorf("B should remain: %v", cfg.Repos["api"].Env)
	}
}

func TestEnvOverrideRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pom.yml")
	os.WriteFile(path, []byte("session: t\nenvironments:\n  staging:\n    API_URL: https://api.staging\n"), 0o644)

	if err := SetEnvOverride(path, "staging", "AI_URL", "https://ai.staging"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := Load(path)
	if cfg.Environments["staging"]["AI_URL"] != "https://ai.staging" {
		t.Fatalf("set failed: %v", cfg.Environments["staging"])
	}

	if err := UnsetEnvOverride(path, "staging", "API_URL"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = Load(path)
	if _, has := cfg.Environments["staging"]["API_URL"]; has {
		t.Fatalf("unset failed: %v", cfg.Environments["staging"])
	}

	if err := SetEnvOverride(path, "prod", "API_URL", "https://api.prod"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = Load(path)
	if cfg.Environments["prod"]["API_URL"] != "https://api.prod" {
		t.Fatalf("new profile failed: %v", cfg.Environments)
	}
}

func TestSetRepoEnv_FileKeyedBase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pom.yml")
	os.WriteFile(path, []byte(
		"session: t\nrepos:\n  api:\n    env:\n      '*':\n        A: '1'\n      .env.local:\n        DB: x\n"), 0o644)

	if err := SetRepoEnv(path, "api", map[string]string{"C": "3"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	starIdx := strings.Index(string(data), "'*':")
	fileIdx := strings.Index(string(data), ".env.local:")
	cIdx := strings.Index(string(data), "C:")
	if !(starIdx < cIdx && cIdx < fileIdx) {
		t.Errorf("C not placed in the '*' base:\n%s", data)
	}
	cfg, _ := Load(path)
	if cfg.Repos["api"].Env["C"] != "3" {
		t.Errorf("resolved base missing C: %v", cfg.Repos["api"].Env)
	}
}
