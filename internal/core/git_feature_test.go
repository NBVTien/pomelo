package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func httptestReq(path string) *http.Request { return httptest.NewRequest(http.MethodGet, path, nil) }

func TestGitRoutes_Mount(t *testing.T) {
	mux := http.NewServeMux()
	(&gitFeature{WorkspaceRoot: ""}).Routes(mux)
	for _, p := range []string{"/api/git/status", "/api/git/checkout", "/api/git/branches"} {
		if h, _ := mux.Handler(httptestReq(p)); h == nil {
			t.Fatalf("route %s not mounted", p)
		}
	}
}
