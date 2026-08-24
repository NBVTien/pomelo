package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceRenameRoundTrip(t *testing.T) {
	root := t.TempDir()
	branch := "proj-1043-widget"
	wsFolder := filepath.Join(root, "workspace--"+branch)
	repoDir := filepath.Join(wsFolder, "api")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	s := &Server{WorkspaceRoot: root, DefaultBranch: "main"}

	set := func(name string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{"branch": branch, "is_main": false, "display_name": name})
		req := httptest.NewRequest(http.MethodPost, "/api/workspace/rename", strings.NewReader(string(body)))
		rec := httptest.NewRecorder()
		s.handleWorkspaceRename(rec, req)
		return rec
	}

	rec := set("Prospect Q widget")
	if rec.Code != http.StatusOK {
		t.Fatalf("rename set: got %d, body=%s", rec.Code, rec.Body.String())
	}

	listed := func() Workspace {
		rec := httptest.NewRecorder()
		s.handleWorkspaces(rec, httptest.NewRequest(http.MethodGet, "/api/workspaces", nil))
		var out struct {
			Workspaces []Workspace `json:"workspaces"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode list: %v", err)
		}
		for _, w := range out.Workspaces {
			if w.Branch == branch {
				return w
			}
		}
		t.Fatalf("workspace %s not in list: %s", branch, rec.Body.String())
		return Workspace{}
	}

	if got := listed().DisplayName; got != "Prospect Q widget" {
		t.Fatalf("display_name after set = %q, want %q", got, "Prospect Q widget")
	}

	if rec := set(""); rec.Code != http.StatusOK {
		t.Fatalf("rename clear: got %d", rec.Code)
	}
	if got := listed().DisplayName; got != "" {
		t.Fatalf("display_name after clear = %q, want empty", got)
	}
}
