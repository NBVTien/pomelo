package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureNodeModulesFromStore(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := t.TempDir()
	lock := []byte("lockfile-contents-v1\n")

	mainWt := filepath.Join(root, "workspace--main", "web")
	_ = os.MkdirAll(filepath.Join(mainWt, "node_modules", "left-pad"), 0o755)
	_ = os.WriteFile(filepath.Join(mainWt, "yarn.lock"), lock, 0o644)
	_ = os.WriteFile(filepath.Join(mainWt, "node_modules", "left-pad", "index.js"), []byte("x"), 0o644)

	target := filepath.Join(root, "workspace--feat", "web")
	_ = os.MkdirAll(target, 0o755)
	_ = os.WriteFile(filepath.Join(target, "yarn.lock"), lock, 0o644)

	if !EnsureNodeModulesFromStore(root, "web", target, "main") {
		t.Fatal("expected store to build from main + populate worktree")
	}
	if !FileExists(filepath.Join(target, "node_modules", "left-pad", "index.js")) {
		t.Fatal("worktree node_modules missing package file")
	}
	if !DirExists(nmStoreDir("web", lockHash(mainWt))) {
		t.Fatal("store not populated from main")
	}
	if EnsureNodeModulesFromStore(root, "web", target, "main") {
		t.Fatal("expected no-op when node_modules already present")
	}
	other := filepath.Join(root, "workspace--feat2", "web")
	_ = os.MkdirAll(other, 0o755)
	_ = os.WriteFile(filepath.Join(other, "yarn.lock"), []byte("lockfile-v2\n"), 0o644)
	if EnsureNodeModulesFromStore(root, "web", other, "main") {
		t.Fatal("expected false when neither store nor main matches the lockfile")
	}
}
