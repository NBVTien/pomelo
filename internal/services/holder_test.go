package services

import (
	"testing"

	"github.com/pomelohq/pomelo/internal/config"
)

func TestWorkspaceHolderMatcher(t *testing.T) {
	cfg := &config.Config{
		Session: "acme",
		Repos: map[string]*config.Dir{
			"api": {Services: map[string]*config.Service{"web": {}}},
		},
		WsServiceOrder: []string{"editor"},
	}
	match := workspaceHolderMatcher(cfg, "investigate-0814",
		[]string{"investigate-0814-1xqh", "investigate-0814-b872"})

	kill := []string{
		"svc-acme-investigate-0814-api-web",
		"ws-acme-investigate-0814-editor",
		"ws-acme-investigate-0814-claude-raw",
		"sh-ws:investigate-0814-shell-abc123",
	}
	for _, n := range kill {
		if !match(n) {
			t.Errorf("expected to kill %q", n)
		}
	}

	keep := []string{
		"ws-acme-investigate-0814-1xqh-claude-raw",
		"sh-ws:investigate-0814-1xqh-shell-def456",
		"sh-ws:investigate-0814-b872-shell-ghi789",
		"ws-acme-other-branch-claude-raw",
		"svc-acme-investigate-0815-api-web",
	}
	for _, n := range keep {
		if match(n) {
			t.Errorf("expected to KEEP %q", n)
		}
	}
}
