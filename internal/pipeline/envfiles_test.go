package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pomelohq/pomelo/internal/config"
)

func TestApplyAllEnvFiles_FileKeyedWithPresetBase(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, ".env.development"), []byte("DATABASE_URL=postgres://localhost:17005/stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := &config.Dir{
		Alias:     "api",
		Databases: map[string]string{"dev": "{{branch.safe}}"},
		Env: map[string]string{
			"REDIS_URL": "redis://localhost:6379/0",
			"MINIO_URL": "http://localhost:9000",
		},
		EnvOutput: []config.EnvFileEntry{
			{File: ".env.development.local", Env: map[string]string{
				"DATABASE_URL": "postgres://localhost:5432/{{db.dev}}",
			}},
		},
	}
	cfg := &config.Config{Session: "acme"}

	applyAllEnvFiles(dir, tmp, cfg, "proj-1043-widget", "ws-proj-1043-widget", "")

	data, err := os.ReadFile(filepath.Join(tmp, ".env.development.local"))
	if err != nil {
		t.Fatalf("env file not written: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "DATABASE_URL=postgres://localhost:5432/acme_") {
		t.Fatalf("DATABASE_URL missing from generated file-keyed env:\n%s", got)
	}
	if strings.Contains(got, "17005") {
		t.Fatalf("stale committed DATABASE_URL leaked instead of the resolved one:\n%s", got)
	}
	if !strings.Contains(got, "REDIS_URL=") || !strings.Contains(got, "MINIO_URL=") {
		t.Fatalf("preset base vars missing:\n%s", got)
	}
}
