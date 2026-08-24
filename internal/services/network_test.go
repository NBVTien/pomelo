package services

import (
	"testing"

	"github.com/pomelohq/pomelo/internal/config"
)

func netTestSetup(t *testing.T) (string, *config.Config) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	projectDir := t.TempDir()
	yes := true
	cfg := &config.Config{
		Session:   t.Name(),
		RepoOrder: []string{"api"},
		Repos: map[string]*config.Dir{
			"api": {
				Alias:        "api",
				ServiceOrder: []string{"web", "worker"},
				Services: map[string]*config.Service{
					"web":    {Cmd: "run web", Port: &yes},
					"worker": {Cmd: "run worker", Port: &yes},
				},
			},
		},
	}
	InitNetwork(projectDir, cfg.Session, cfg)
	return projectDir, cfg
}

func TestPortStickyAfterAcquire(t *testing.T) {
	dir, cfg := netTestSetup(t)
	if p := Port(dir, "ws-alpha", "api~web"); p != 0 {
		t.Fatalf("port assigned before acquire: %d", p)
	}
	acquireWorkspacePorts(cfg, "ws-alpha")

	p1 := Port(dir, "ws-alpha", "api~web")
	if p1 < portLo || p1 > portHi {
		t.Fatalf("web port out of range: %d", p1)
	}
	if p2 := Port(dir, "ws-alpha", "api~web"); p2 != p1 {
		t.Fatalf("port not sticky: %d vs %d", p1, p2)
	}
	if q := Port(dir, "ws-alpha", "api~worker"); q == p1 || q == 0 {
		t.Fatalf("distinct services must get distinct ports: %d vs %d", p1, q)
	}
}

func TestPortsDistinctAcrossWorkspaces(t *testing.T) {
	dir, cfg := netTestSetup(t)
	acquireWorkspacePorts(cfg, "ws-one")
	acquireWorkspacePorts(cfg, "ws-two")
	if p1, p2 := Port(dir, "ws-one", "api~web"), Port(dir, "ws-two", "api~web"); p1 == 0 || p1 == p2 {
		t.Fatalf("same service in different workspaces shared port %d", p1)
	}
}

func TestReleaseBlockFreesPorts(t *testing.T) {
	dir, cfg := netTestSetup(t)
	acquireWorkspacePorts(cfg, "ws-gone")
	if Port(dir, "ws-gone", "api~web") == 0 {
		t.Fatal("expected a port after acquire")
	}
	ReleaseBlock(dir, "ws-gone")
	mgr().Snapshot()
	if p := Port(dir, "ws-gone", "api~web"); p != 0 {
		t.Fatalf("port survived release: %d", p)
	}
}

func TestPreflightPortUsable(t *testing.T) {
	dir, _ := netTestSetup(t)
	p, err := PreflightPort(dir, "ws-pf", "api~web")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == 0 || !IsPortFree(p) {
		t.Fatalf("preflight returned an unusable port: %d", p)
	}
}
