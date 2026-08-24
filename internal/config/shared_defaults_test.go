package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWellKnownSharedDefaults(t *testing.T) {
	dir := t.TempDir()
	yml := `session: demo
repos:
  api:
    services:
      s:
        cmd: x
shared_services:
  postgres:
  redis:
    capacity: 8
  cache:
    type: redis
`
	p := filepath.Join(dir, "pom.yml")
	if err := os.WriteFile(p, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}

	pg := cfg.SharedServices["postgres"]
	if pg == nil || pg.Image != "postgres:16" || len(pg.Ports) != 1 || pg.Ports[0] != "5432" {
		t.Fatalf("postgres defaults not applied: %+v", pg)
	}
	if pg.Environment["POSTGRES_USER"] != "postgres" || pg.Healthcheck == nil || pg.DBUser != "postgres" {
		t.Fatalf("postgres env/healthcheck/db defaults missing: %+v", pg)
	}

	r := cfg.SharedServices["redis"]
	if r.Image != "redis:7-alpine" || r.Capacity == nil || *r.Capacity != 8 {
		t.Fatalf("redis override/default wrong: image=%q cap=%v", r.Image, r.Capacity)
	}

	c := cfg.SharedServices["cache"]
	if c.Image != "redis:7-alpine" || c.Command != "redis-server --appendonly yes" {
		t.Fatalf("type-based template not applied: %+v", c)
	}
}
