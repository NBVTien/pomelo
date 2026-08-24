package services

import "testing"

import "github.com/pomelohq/pomelo/internal/config"

func explainCfg() *config.Config {
	cfg := &config.Config{
		DefaultBranch: "main",
		Repos: map[string]*config.Dir{
			"api": {Alias: "api", Services: map[string]*config.Service{
				"web": {},
			}},
		},
		Environments: map[string]map[string]string{
			"local":   {},
			"staging": {"api.web": "https://api.staging.example.com"},
		},
	}
	return cfg
}

func TestExplainService(t *testing.T) {
	cfg := explainCfg()
	cfg.Repos["api"].Env = map[string]string{
		"AI_SERVICE_URL": "{{api.web.url}}",
		"PORT_HINT":      "{{branch.safe}}",
	}
	cfg.Repos["api"].Services["web"].Cmd = "bundle exec puma"
	cfg.Repos["api"].ServiceOrder = []string{"web"}

	se, err := ExplainService(cfg, "api", "web", "feat/x", "staging")
	if err != nil {
		t.Fatal(err)
	}
	if se.Repo != "api" || se.Service != "web" || se.Cmd != "bundle exec puma" {
		t.Fatalf("head = %+v", se)
	}
	env := map[string]string{}
	for _, e := range se.Env {
		env[e.Key] = e.Value
	}
	if env["AI_SERVICE_URL"] != "https://api.staging.example.com" {
		t.Errorf("AI_SERVICE_URL = %q, want the staging api.web override", env["AI_SERVICE_URL"])
	}
	if env["PORT_HINT"] != "feat_x" {
		t.Errorf("PORT_HINT = %q, want feat_x (branch.safe)", env["PORT_HINT"])
	}

	if _, err := ExplainService(cfg, "nope", "web", "main", ""); err == nil {
		t.Error("expected error for unknown repo")
	}
}
