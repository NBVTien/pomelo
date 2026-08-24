package config

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestAnalyzeDuplicateEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pom.yml")
	yaml := "session: t\n" +
		"repos:\n" +
		"  api:\n    env:\n      LOG: info\n      DB: api_db\n" +
		"  web:\n    env:\n      LOG: info\n      DB: web_db\n" +
		"  worker:\n    env:\n      LOG: info\n"
	os.WriteFile(path, []byte(yaml), 0o644)

	dups, err := AnalyzeDuplicateEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(dups) != 1 {
		t.Fatalf("got %d dups, want 1: %+v", len(dups), dups)
	}
	d := dups[0]
	if d.Key != "LOG" || d.Value != "info" || len(d.Repos) != 3 {
		t.Errorf("dup = %+v, want LOG=info across 3 repos", d)
	}
}

func TestLintDeprecatedKeys(t *testing.T) {
	dir := t.TempDir()

	dep := filepath.Join(dir, "a.yml")
	os.WriteFile(dep, []byte("session: t\ncombinations:\n  full: [api, web]\n"), 0o644)
	got, err := LintDeprecatedKeys(dep)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.Contains(got[0], "combinations") {
		t.Fatalf("got %v, want a combinations deprecation", got)
	}

	clean := filepath.Join(dir, "b.yml")
	os.WriteFile(clean, []byte("session: t\nworkspaces:\n  full: [api, web]\n"), 0o644)
	if got, _ := LintDeprecatedKeys(clean); len(got) != 0 {
		t.Errorf("workspaces should be clean, got %v", got)
	}
}

func TestMigrateTokens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pom.yml")
	orig := "session: t\nrepos:\n  api:\n    databases:\n      a: \"{{branch_safe}}\"\n      b: \"{{branch_hash}}\"\n"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}

	changes, err := MigrateTokens(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if changes[path] != 2 {
		t.Fatalf("dry-run count = %d, want 2", changes[path])
	}
	if data, _ := os.ReadFile(path); string(data) != orig {
		t.Fatal("dry run must not modify the file")
	}

	if _, err := MigrateTokens(path, true); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	got := string(data)
	if strings.Contains(got, "{{branch_safe}}") || strings.Contains(got, "{{branch_hash}}") {
		t.Errorf("legacy tokens remain after write: %s", got)
	}
	if !strings.Contains(got, "{{branch|safe}}") || !strings.Contains(got, "{{branch|hash}}") {
		t.Errorf("filter form missing after write: %s", got)
	}
}

func TestLintLegacyTokens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pom.yml")
	yaml := "session: t\nrepos:\n  api:\n    databases:\n      a: \"{{branch_safe}}\"\n      b: \"{{branch_hash}}_{{branch_safe}}\"\n"
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LintLegacyTokens(path)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, lt := range got {
		counts[lt.Old] = lt.Count
	}
	if counts["{{branch_safe}}"] != 2 || counts["{{branch_hash}}"] != 1 {
		t.Errorf("counts = %v, want branch_safe:2 branch_hash:1", counts)
	}
	if _, ok := counts["{{branch_host}}"]; ok {
		t.Error("branch_host not present, should not be reported")
	}
}

func TestRedundantSharedDefaults(t *testing.T) {
	cfg := &Config{SharedServices: map[string]*SharedServiceDef{
		"postgres": {
			Image:      "postgres:16",
			DBUser:     "postgres",
			DBPassword: "postgres",
			Command:    "postgres -c custom",
		},
		"cache": {
			Type:  "redis",
			Image: "redis:6",
			Ports: []string{"6379"},
		},
		"custom": {Image: "acme/thing"},
	}}

	got := redundantSharedDefaults(cfg)

	pg := got["postgres"]
	sort.Strings(pg)
	if !reflect.DeepEqual(pg, []string{"db_password", "db_user", "image"}) {
		t.Errorf("postgres redundant = %v, want [db_password db_user image]", pg)
	}
	if got["cache"] == nil || !reflect.DeepEqual(got["cache"], []string{"ports"}) {
		t.Errorf("cache redundant = %v, want [ports]", got["cache"])
	}
	if _, ok := got["custom"]; ok {
		t.Errorf("unknown service should not be reported, got %v", got["custom"])
	}
}

func TestRedundantSharedDefaults_EnvEntries(t *testing.T) {
	cfg := &Config{SharedServices: map[string]*SharedServiceDef{
		"postgres": {Environment: map[string]string{
			"POSTGRES_USER": "postgres",
			"POSTGRES_DB":   "myapp",
		}},
	}}
	got := redundantSharedDefaults(cfg)
	if !reflect.DeepEqual(got["postgres"], []string{"environment.POSTGRES_USER"}) {
		t.Errorf("env redundant = %v, want [environment.POSTGRES_USER]", got["postgres"])
	}
}

func TestLintInlineCmdEnv(t *testing.T) {
	cfg := &Config{
		RepoOrder: []string{"api"},
		Repos: map[string]*Dir{
			"api": {Alias: "api", ServiceOrder: []string{"web", "clean"}, Services: map[string]*Service{
				"web":   {Cmd: "RACK_ENV=production WEB_CONCURRENCY=0 bundle exec puma"},
				"clean": {Cmd: "bundle exec puma"},
			}},
		},
	}
	got := LintInlineCmdEnv(cfg)
	if len(got) != 1 {
		t.Fatalf("got %d, want 1 (only web has inline env): %+v", len(got), got)
	}
	if got[0].Service != "api/web" || len(got[0].Keys) != 2 {
		t.Errorf("got %+v, want api/web with RACK_ENV+WEB_CONCURRENCY", got[0])
	}
}
