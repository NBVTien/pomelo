package pipeline

import (
	"testing"

	"github.com/pomelohq/pomelo/internal/config"
	"github.com/pomelohq/pomelo/internal/services"
)

func TestFindDirPath(t *testing.T) {
	ctx := &CreateContext{DirPaths: []services.DirMapping{
		{Name: "api", Path: "/ws/api"},
		{Name: "web", Path: "/ws/web"},
	}}
	if got := findDirPath(ctx, "web"); got != "/ws/web" {
		t.Errorf("findDirPath(web) = %q, want /ws/web", got)
	}
	if got := findDirPath(ctx, "missing"); got != "" {
		t.Errorf("findDirPath(missing) = %q, want empty", got)
	}
}

func TestResolveTargetBranch(t *testing.T) {
	base := &CreateContext{Branch: "feat/x"}
	if got := resolveTargetBranch(base, "api"); got != "feat/x" {
		t.Errorf("no selection → %q, want feat/x", got)
	}

	sel := &CreateContext{Branch: "feat/x", SelectedDirs: []services.DirBranch{
		{Name: "api", Branch: "main"},
		{Name: "web", Branch: "feat/x"},
	}}
	if got := resolveTargetBranch(sel, "api"); got != "main" {
		t.Errorf("api override → %q, want main", got)
	}
	if got := resolveTargetBranch(sel, "web"); got != "feat/x" {
		t.Errorf("web same-branch → %q, want feat/x", got)
	}
	if got := resolveTargetBranch(sel, "other"); got != "feat/x" {
		t.Errorf("unselected repo → %q, want feat/x", got)
	}
}

func TestFindPGService(t *testing.T) {
	cfg := &config.Config{SharedServices: map[string]*config.SharedServiceDef{
		"redis":    {},
		"postgres": {DBUser: "postgres"},
	}}
	if got := findPGService(cfg); got == nil || got.DBUser != "postgres" {
		t.Errorf("findPGService = %+v, want the postgres def", got)
	}

	none := &config.Config{SharedServices: map[string]*config.SharedServiceDef{"redis": {}}}
	if got := findPGService(none); got != nil {
		t.Errorf("findPGService(no db user) = %+v, want nil", got)
	}
}
