package codeagent

import (
	"testing"

	"github.com/pomelohq/pomelo/internal/config"
)

func TestInject_AddsBuiltinAgents(t *testing.T) {
	cfg := &config.Config{}
	Inject(cfg)

	svc, ok := cfg.WsServices["Claude Code"]
	if !ok {
		t.Fatal("Claude Code service not injected")
	}
	if svc.Cmd != "claude" {
		t.Errorf("got cmd %q, want %q", svc.Cmd, "claude")
	}
	if svc.Port == nil || *svc.Port {
		t.Errorf("Claude Code should have Port=false")
	}
	if len(cfg.WsServiceOrder) != 1 || cfg.WsServiceOrder[0] != "Claude Code" {
		t.Errorf("WsServiceOrder = %v, want [Claude Code]", cfg.WsServiceOrder)
	}
}

func TestInject_PreservesUserDefinedService(t *testing.T) {
	userCmd := "my-custom-claude"
	userPort := true
	cfg := &config.Config{
		WsServices: map[string]*config.Service{
			"Claude Code": {Cmd: userCmd, Port: &userPort},
		},
		WsServiceOrder: []string{"Claude Code"},
	}
	Inject(cfg)

	svc := cfg.WsServices["Claude Code"]
	if svc.Cmd != userCmd {
		t.Errorf("user service overwritten: got %q, want %q", svc.Cmd, userCmd)
	}
	if len(cfg.WsServiceOrder) != 1 {
		t.Errorf("WsServiceOrder mutated: %v", cfg.WsServiceOrder)
	}
}

func TestInject_DisabledOptOut(t *testing.T) {
	cfg := &config.Config{
		CodeAgents: &config.CodeAgentsConfig{Disabled: true},
	}
	Inject(cfg)
	if len(cfg.WsServices) != 0 {
		t.Errorf("disabled inject still added services: %v", cfg.WsServices)
	}
}

func TestInject_OnlyWhitelist(t *testing.T) {
	cfg := &config.Config{
		CodeAgents: &config.CodeAgentsConfig{Only: []string{"NonExistent"}},
	}
	Inject(cfg)
	if _, ok := cfg.WsServices["Claude Code"]; ok {
		t.Errorf("Claude Code should be filtered out when not in Only list")
	}
}

func TestInject_NilConfigSafe(t *testing.T) {
	Inject(nil)
}

func TestLookupAgent(t *testing.T) {
	if a := LookupAgent("Claude Code"); a == nil || a.Cmd != "claude" {
		t.Errorf("LookupAgent failed: %v", a)
	}
	if a := LookupAgent("Unknown"); a != nil {
		t.Errorf("LookupAgent should return nil for unknown: %v", a)
	}
}
