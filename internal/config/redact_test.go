package config

import (
	"strings"
	"testing"
)

func TestRedactYAML(t *testing.T) {
	in := `jira:
  site: https://acme.atlassian.net
  email: me@acme.com
  token_env: JIRA_API_TOKEN
repos:
  api:
    environments: [local, staging]
    env:
      STRIPE_PK: pk_test_secret123
      PUBLIC_HOST: myhost
environments:
  staging:
    API_URL: https://api.staging.internal
`
	out, err := RedactYAML([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, must := range []string{"email: <redacted>", "site: <redacted>", "STRIPE_PK: <redacted>", "API_URL: <redacted>"} {
		if !strings.Contains(s, must) {
			t.Errorf("expected %q in:\n%s", must, s)
		}
	}
	if !strings.Contains(s, "JIRA_API_TOKEN") {
		t.Error("token_env value (a var name) must not be redacted")
	}
	if !strings.Contains(s, "local") || !strings.Contains(s, "staging") {
		t.Error("repo-level environments list (profile names) must be kept")
	}
	if !strings.Contains(s, "staging:") || !strings.Contains(s, "API_URL:") {
		t.Error("top-level environment profile + var names must be kept")
	}
	if strings.Contains(s, "myhost") == false {
		t.Error("non-sensitive env value should be kept")
	}
}
