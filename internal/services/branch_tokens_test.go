package services

import "testing"

func TestResolveBranchTokens_BothSyntaxes(t *testing.T) {
	branch := "feat/x"
	safe := BranchSafe(branch)
	host := BranchHost(branch)
	hash := BranchHash(branch)

	cases := map[string]string{
		"{{branch}}":             branch,
		"{{branch|safe}}":        safe,
		"{{branch_safe}}":        safe,
		"{{branch|host}}":        host,
		"{{branch_host}}":        host,
		"{{branch|hash}}":        hash,
		"{{branch_hash}}":        hash,
		"db_{{branch|safe}}_app": "db_" + safe + "_app",
		"no tokens here":         "no tokens here",
	}
	for in, want := range cases {
		if got := ResolveBranchTokens(in, branch); got != want {
			t.Errorf("ResolveBranchTokens(%q) = %q, want %q", in, got, want)
		}
	}

	mixed := ResolveBranchTokens("{{branch|safe}}-{{branch_safe}}", branch)
	if mixed != safe+"-"+safe {
		t.Errorf("mixed = %q, want %q", mixed, safe+"-"+safe)
	}
}
