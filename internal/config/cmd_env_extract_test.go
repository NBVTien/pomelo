package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplitLeadingEnv(t *testing.T) {
	cases := []struct {
		in   string
		keys int
		bare string
		ok   bool
	}{
		{"RACK_ENV=prod WEB_CONCURRENCY=0 bundle exec puma", 2, "bundle exec puma", true},
		{"bundle exec puma", 0, "", false},
		{"DATABASE_URL=postgres://x/y bundle exec rake", 1, "bundle exec rake", true},
		{"FOO='a b' bundle", 0, "", false},
		{"RACK_ENV=prod", 0, "", false},
	}
	for _, c := range cases {
		pairs, bare, ok := splitLeadingEnv(c.in)
		if ok != c.ok || (ok && (len(pairs) != c.keys || bare != c.bare)) {
			t.Errorf("splitLeadingEnv(%q) = (%d pairs, %q, %v), want (%d, %q, %v)",
				c.in, len(pairs), bare, ok, c.keys, c.bare, c.ok)
		}
	}
}

func TestExtractCmdEnv_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "pom.yml")
	os.WriteFile(p, []byte("session: t\nrepos:\n  api:\n    services:\n"+
		"      web:\n        cmd: RACK_ENV=prod WEB_CONCURRENCY=0 bundle exec puma -p $PORT\n"+
		"      worker: RACK_ENV=prod bundle exec sidekiq\n"), 0o644)

	plan, err := ExtractCmdEnv(p, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 {
		t.Fatalf("plan = %v, want 2 services", plan)
	}
	cfg, _ := Load(p)
	web := cfg.Repos["api"].Services["web"]
	if web.Cmd != "bundle exec puma -p $PORT" || web.Env["RACK_ENV"] != "prod" || web.Env["WEB_CONCURRENCY"] != "0" {
		t.Errorf("web = cmd:%q env:%v", web.Cmd, web.Env)
	}
	wk := cfg.Repos["api"].Services["worker"]
	if wk.Cmd != "bundle exec sidekiq" || wk.Env["RACK_ENV"] != "prod" {
		t.Errorf("worker = cmd:%q env:%v", wk.Cmd, wk.Env)
	}
}
