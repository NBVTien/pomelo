package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeStripsRemovedKeysAndMigratesColonTokens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pom.yml")
	src := `session: demo
default_branch: main
schema_version: 2
combinations:
  all: [api]
proxy:
  app: api
repos:
  api:
    alias: api
    plugins:
      e2e:
        db: x
    env:
      "*":
        DB_URL: postgres://{{conn:postgres}}/{{db:main}}
        HOST: "{{host:redis}}"
    services:
      web:
        cmd: run
        exposes: [API_URL]
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	if rk := RemovedKeys(path); len(rk) == 0 {
		t.Fatalf("expected removed keys before normalize, got none")
	}

	removed, err := Normalize(path)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if len(removed) == 0 {
		t.Fatalf("expected Normalize to report removed keys")
	}

	if rk := RemovedKeys(path); len(rk) != 0 {
		t.Fatalf("removed keys still present after normalize: %v", rk)
	}
	merged, _, err := MergedYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	m := string(merged)
	for _, bad := range []string{"schema_version", "combinations", "proxy", "plugins", "exposes", "{{conn:", "{{db:", "{{host:"} {
		if strings.Contains(m, bad) {
			t.Errorf("normalized config still contains %q:\n%s", bad, m)
		}
	}
	for _, want := range []string{"{{shared.postgres.url}}", "{{db.main}}", "{{shared.redis.host}}"} {
		if !strings.Contains(m, want) {
			t.Errorf("expected migrated token %q in:\n%s", want, m)
		}
	}
}
