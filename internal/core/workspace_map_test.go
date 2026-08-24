package core

import (
	"strings"
	"testing"

	"github.com/pomelohq/pomelo/internal/config"
)

func TestBuildWorkspaceMap(t *testing.T) {
	cfg := &config.Config{
		Session:   "demo",
		RepoOrder: []string{"api-repo", "web-repo"},
		Repos: map[string]*config.Dir{
			"api-repo": {
				Alias:     "api",
				Databases: map[string]string{"main": "app"},
				Profiles:  config.StringList{"local", "staging"},
				Services: map[string]*config.Service{
					"api": {},
				},
			},
			"web-repo": {
				Alias: "web",
				Services: map[string]*config.Service{
					"portal": {},
				},
			},
		},
	}

	out := buildWorkspaceMap(cfg, "feat-login")

	for _, want := range []string{
		workspaceMapMarker,
		"# demo workspace · feat-login",
		"**api** — `api-repo/`",
		"db: main",
		"profiles: local, staging",
		"**web** — `web-repo/`",
		"scope each change to ONE repo",
		"pom` MCP",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("map missing %q\n---\n%s", want, out)
		}
	}

	if strings.Index(out, "api-repo/") > strings.Index(out, "web-repo/") {
		t.Errorf("repos not in RepoOrder:\n%s", out)
	}
}

func TestBuildWorkspaceMapSingleRepoStillBuilds(t *testing.T) {
	cfg := &config.Config{
		Session: "solo",
		Repos:   map[string]*config.Dir{"only": {Alias: "only"}},
	}
	if out := buildWorkspaceMap(cfg, "b"); !strings.Contains(out, "**only**") {
		t.Errorf("unexpected: %s", out)
	}
}
