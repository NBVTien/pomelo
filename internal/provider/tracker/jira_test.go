package tracker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pomelohq/pomelo/internal/config"
)

func registerJira(mux *http.ServeMux) {
	NewJira(func() *config.Config { return nil }).Routes(mux)
}

func TestJiraRoutes_UnconfiguredBehaviour(t *testing.T) {
	mux := http.NewServeMux()
	registerJira(mux)

	t.Run("issues → configured:false", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/jira/issues", strings.NewReader(`{"branches":["proj-1"]}`))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("bad json: %v (%s)", err, w.Body.String())
		}
		if body["configured"] != false {
			t.Fatalf("want configured:false, got %v", body)
		}
	})

	t.Run("test → ok:false with a message", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/jira/test", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("bad json: %v (%s)", err, w.Body.String())
		}
		if body["ok"] != false || body["error"] == nil {
			t.Fatalf("want ok:false + error, got %v", body)
		}
	})
}
