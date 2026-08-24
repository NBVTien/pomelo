package core

import (
	"net/http"
	"strings"
	"testing"
)

func rewrite(prefix string, cookies ...string) []string {
	resp := &http.Response{Header: http.Header{"Set-Cookie": cookies}}
	rewriteSetCookie(resp, prefix)
	return resp.Header["Set-Cookie"]
}

func TestRewriteSetCookiePrependsPrefixAndDropsSecure(t *testing.T) {
	out := rewrite("/be/api",
		"portal_refresh_token=abc; Path=/portal/v1/auth/jwt; Secure; HttpOnly; SameSite=Lax")[0]
	if !strings.Contains(out, "Path=/be/api/portal/v1/auth/jwt") {
		t.Fatalf("path not prefixed: %q", out)
	}
	if strings.Contains(strings.ToLower(out), "secure") {
		t.Fatalf("Secure not stripped: %q", out)
	}
	if !strings.Contains(out, "SameSite=Lax") || !strings.Contains(out, "HttpOnly") {
		t.Fatalf("other attrs mangled: %q", out)
	}
}

func TestRewriteSetCookieNoPrefixJustStripsSecure(t *testing.T) {
	for _, prefix := range []string{"", "/"} {
		out := rewrite(prefix, "s=1; Path=/x; Secure")[0]
		if !strings.Contains(out, "Path=/x") {
			t.Errorf("prefix %q: path changed: %q", prefix, out)
		}
		if strings.Contains(strings.ToLower(out), "secure") {
			t.Errorf("prefix %q: Secure not stripped: %q", prefix, out)
		}
	}
}

func TestRewriteSetCookieNoCookies(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	rewriteSetCookie(resp, "/be/api")
	if len(resp.Header["Set-Cookie"]) != 0 {
		t.Fatal("unexpected cookies")
	}
}
