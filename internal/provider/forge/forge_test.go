package forge

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pomelohq/pomelo/internal/config"
)

func TestPRRoutes_Behaviour(t *testing.T) {
	mux := http.NewServeMux()
	New(func() *config.Config { return nil }, "", "main").Routes(mux)

	for _, path := range []string{"/api/repo/pr", "/api/repo/commits"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: want 400 without branch/repo, got %d", path, w.Code)
		}
	}
}
