package arch

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const modPath = "github.com/pomelohq/pomelo"

var rank = map[string]int{
	"config":   0,
	"paths":    0,
	"lock":     0,
	"plugin":   0,
	"ptyhost":  1,
	"services": 1,
	"pipeline": 2,
	"jira":     2,
	"agent":    2,
	"sessions": 2,
	"commands": 3,
	"mcp":      3,
	"web":      4,
}

func TestBackendLayeringInwardOnly(t *testing.T) {
	root := moduleRoot(t)
	internal := filepath.Join(root, "internal")

	var violations []string
	for pkg, srcRank := range rank {
		imports := internalImports(t, filepath.Join(internal, pkg))
		for _, imp := range imports {
			dstRank, known := rank[imp]
			if !known {
				continue
			}
			if srcRank == 0 && imp != pkg {
				violations = append(violations, "core "+pkg+" imports internal/"+imp+" (core must be pure)")
				continue
			}
			if dstRank > srcRank {
				violations = append(violations,
					"internal/"+pkg+" (rank "+itoa(srcRank)+") imports internal/"+imp+" (rank "+itoa(dstRank)+") — outward leak")
			}
		}
	}

	if len(violations) > 0 {
		t.Errorf("backend layering violations (%d):\n  %s\n\nSee docs/ARCHITECTURE.md. Fix by moving logic inward or inverting the dependency.",
			len(violations), strings.Join(violations, "\n  "))
	}
}

func internalImports(t *testing.T, dir string) []string {
	fset := token.NewFileSet()
	seen := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		for _, spec := range f.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			if sub, ok := strings.CutPrefix(path, modPath+"/internal/"); ok {
				seen[strings.SplitN(sub, "/", 2)[0]] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	return out
}

func moduleRoot(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func itoa(n int) string { return string(rune('0' + n)) }
