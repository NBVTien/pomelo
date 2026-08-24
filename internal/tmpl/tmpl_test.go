package tmpl

import (
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	lookup := func(k string) (string, bool) {
		m := map[string]string{
			"branch":             "proj-1147",
			"service.api.server": "http://server.api.proj-1147.localhost:8767",
			"shared.postgres":    "user:pw@localhost:5432",
			"db.main":            "acme_proj_1147",
		}
		v, ok := m[k]
		return v, ok
	}
	filters := map[string]Filter{
		"safe": func(s string) string { return strings.ReplaceAll(s, "-", "_") },
		"ws":   func(s string) string { return strings.Replace(s, "http", "ws", 1) },
		"port": func(s string) string { i := strings.LastIndexByte(s, ':'); return s[i+1:] },
	}
	cases := []struct{ in, want string }{
		{"{{branch}}", "proj-1147"},
		{"{{branch|safe}}", "proj_1147"},
		{"pre-{{branch}}-post", "pre-proj-1147-post"},
		{"{{service.api.server}}", "http://server.api.proj-1147.localhost:8767"},
		{"{{service.api.server|ws}}", "ws://server.api.proj-1147.localhost:8767"},
		{"{{service.api.server|port}}", "8767"},
		{"DB={{db.main}}", "DB=acme_proj_1147"},
		{"{{unknown.key}}", "{{unknown.key}}"},
		{"no tokens", "no tokens"},
		{"{{shared.postgres}}", "user:pw@localhost:5432"},
	}
	for _, c := range cases {
		if got := Resolve(c.in, lookup, filters); got != c.want {
			t.Errorf("Resolve(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRefs(t *testing.T) {
	got := Refs("{{branch|safe}} and {{service.api.server}} and {{branch}}")
	want := []string{"branch", "service.api.server"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Refs = %v, want %v", got, want)
	}
}
