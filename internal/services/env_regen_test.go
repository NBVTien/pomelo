package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pomelohq/pomelo/internal/config"
)

func boolPtr(b bool) *bool { return &b }

func TestRegenerateWorkspaceEnv(t *testing.T) {
	projectDir := t.TempDir()
	cfg := &config.Config{
		Session:   "test-regen",
		RepoOrder: []string{"my-api"},
		Repos: map[string]*config.Dir{
			"my-api": {
				Alias:        "api",
				ServiceOrder: []string{"server", "worker"},
				Services: map[string]*config.Service{
					"server": {Cmd: "go run .", Port: boolPtr(true)},
					"worker": {Cmd: "go run ./worker", Port: boolPtr(true)},
				},
			},
		},
		SharedServices: map[string]*config.SharedServiceDef{
			"postgres": {Ports: []string{"5432"}},
		},
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	InitNetwork(projectDir, cfg.Session, cfg)

	wsKey := "ws-main"

	wsFolder := filepath.Join(projectDir, "workspace--main", "my-api")
	_ = os.MkdirAll(wsFolder, 0o755)
	_ = os.WriteFile(filepath.Join(wsFolder, ".env"), []byte("PORT=0\n"), 0o644)

	RegenerateWorkspaceEnv(projectDir, cfg, "main")

	serverPort := Port(projectDir, wsKey, "api~server")
	workerPort := Port(projectDir, wsKey, "api~worker")
	if serverPort == 0 || workerPort == 0 {
		t.Fatalf("ports not allocated after env regen: server=%d worker=%d", serverPort, workerPort)
	}
	if serverPort == workerPort {
		t.Fatalf("distinct services must get distinct ports: %d", serverPort)
	}
	t.Logf("server=%d worker=%d", serverPort, workerPort)
}

func TestRegenerateWorkspaceEnv_PreservesFileKeyedDBOnServiceWrite(t *testing.T) {
	projectDir := t.TempDir()
	cfg := &config.Config{
		Session:   "regen-db",
		RepoOrder: []string{"my-api"},
		Repos: map[string]*config.Dir{
			"my-api": {
				Alias:        "api",
				Databases:    map[string]string{"dev": "{{branch|safe}}"},
				ServiceOrder: []string{"api"},
				Env:          map[string]string{"SHARED_BASE": "1"},
				EnvOutput: []config.EnvFileEntry{
					{File: ".env.development.local", Env: map[string]string{
						"DATABASE_URL": "postgres://{{shared.postgres.url}}/{{db.dev}}",
					}},
				},
				Services: map[string]*config.Service{
					"api": {Cmd: "puma", Env: map[string]string{"RACK_ENV": "development"}},
				},
			},
		},
		SharedServices: map[string]*config.SharedServiceDef{
			"postgres": {Ports: []string{"5432"}, DBUser: "u", DBPassword: "p"},
		},
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	InitNetwork(projectDir, cfg.Session, cfg)

	wtPath := filepath.Join(projectDir, "workspace--main", "my-api")
	_ = os.MkdirAll(wtPath, 0o755)
	_ = os.WriteFile(filepath.Join(wtPath, ".env.development"), []byte("DATABASE_URL=postgres://stale\n"), 0o644)

	RegenerateWorkspaceEnv(projectDir, cfg, "main")

	data, err := os.ReadFile(filepath.Join(wtPath, ".env.development.local"))
	if err != nil {
		t.Fatalf("env file not written: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "DATABASE_URL=postgres://") || strings.Contains(got, "{{") {
		t.Fatalf("DATABASE_URL missing/unresolved after service-env write:\n%s", got)
	}
	if !strings.Contains(got, "RACK_ENV=development") {
		t.Fatalf("service env RACK_ENV missing:\n%s", got)
	}
}
