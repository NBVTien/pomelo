package claude

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pomelohq/pomelo/internal/config"
)

func httptestReq(path string) *http.Request { return httptest.NewRequest(http.MethodGet, path, nil) }

func TestClaudeRoutes_Mount(t *testing.T) {
	f := &Feature{
		getCfg:              func() *config.Config { return nil },
		mcpConfigJSONFn:     func(string) string { return "" },
		writeWorkspaceMapFn: func(string, bool) {},
		ticketContextFn:     func(string) string { return "" },
		headless:            map[string]*Driver{},
		agentSubs:           map[chan []byte]struct{}{},
		agentState:          map[string]string{},
	}
	mux := http.NewServeMux()
	f.Routes(mux)
	for _, p := range []string{"/api/claude/session", "/api/claude/commands", "/ws/claude/stream", "/api/agents/stream", "/api/sessions"} {
		if h, _ := mux.Handler(httptestReq(p)); h == nil {
			t.Fatalf("route %s not mounted", p)
		}
	}
}
