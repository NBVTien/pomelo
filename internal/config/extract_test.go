package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractPreset_HoistsAndPreservesResolved(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pom.yml"), []byte("session: t\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "pom.d"), 0o755)
	os.WriteFile(filepath.Join(dir, "pom.d", "presets.yml"), []byte("presets:\n  base:\n    env:\n      X: '1'\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "pom.d", "repos.yml"), []byte(
		"repos:\n"+
			"  api:\n    preset: base\n    env:\n      SHARED: same\n      OWN: a\n"+
			"  web:\n    env:\n      SHARED: same\n      OWN: b\n"), 0o644)

	path := filepath.Join(dir, "pom.yml")

	plan, err := PlanExtractPreset(path, "common", []string{"SHARED"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Repos) != 2 || plan.Keys["SHARED"] != "same" {
		t.Fatalf("plan = %+v, want SHARED across api+web", plan)
	}

	before, _ := resolvedEnvs(path, plan.Repos)

	if err := ApplyExtractPreset(path, plan); err != nil {
		t.Fatal(err)
	}

	repos, _ := os.ReadFile(filepath.Join(dir, "pom.d", "repos.yml"))
	if strings.Count(string(repos), "SHARED: same") != 0 {
		t.Errorf("SHARED not removed from repos:\n%s", repos)
	}
	if !strings.Contains(string(repos), "common") {
		t.Errorf("preset ref `common` not added:\n%s", repos)
	}
	presets, _ := os.ReadFile(filepath.Join(dir, "pom.d", "presets.yml"))
	if !strings.Contains(string(presets), "common") || !strings.Contains(string(presets), "SHARED: same") {
		t.Errorf("preset `common` with SHARED not created:\n%s", presets)
	}

	after, _ := resolvedEnvs(path, plan.Repos)
	for _, r := range plan.Repos {
		if !envEqual(before[r], after[r]) {
			t.Errorf("resolved env changed for %s: before=%v after=%v", r, before[r], after[r])
		}
	}
	if after["api"]["OWN"] != "a" || after["web"]["OWN"] != "b" {
		t.Errorf("per-repo OWN lost: %v / %v", after["api"], after["web"])
	}
}
