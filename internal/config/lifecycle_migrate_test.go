package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGroupLifecycle_MovesAndPreserves(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pom.yml"), []byte("session: t\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "pom.d"), 0o755)
	repoFile := filepath.Join(dir, "pom.d", "repos.yml")
	os.WriteFile(repoFile, []byte(
		"repos:\n"+
			"  api:\n"+
			"    alias: api\n"+
			"    setup: [bundle install]\n"+
			"    seed: [rake seed]\n"+
			"    services:\n      web:\n        cmd: run\n"), 0o644)
	path := filepath.Join(dir, "pom.yml")

	before, _ := lifecycleSnapshot(path)

	plan, err := GroupLifecycle(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if plan["api"] != 2 {
		t.Fatalf("plan = %v, want api:2", plan)
	}

	out, _ := os.ReadFile(repoFile)
	s := string(out)
	if !strings.Contains(s, "lifecycle:") {
		t.Errorf("no lifecycle block:\n%s", s)
	}
	if strings.Index(s, "lifecycle:") > strings.Index(s, "setup:") {
		t.Errorf("setup should be under lifecycle:\n%s", s)
	}
	if !strings.Contains(s, "alias: api") || !strings.Contains(s, "services:") {
		t.Errorf("identity fields lost:\n%s", s)
	}

	after, _ := lifecycleSnapshot(path)
	if before != after {
		t.Errorf("effective lifecycle changed:\nbefore=%s\nafter=%s", before, after)
	}
}
