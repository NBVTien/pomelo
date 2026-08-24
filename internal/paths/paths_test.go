package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateDir_XDG(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	t.Setenv("HOME", tmp)

	dir := StateDir()
	want := filepath.Join(tmp, "pom")
	if dir != want {
		t.Errorf("StateDir() = %q, want %q", dir, want)
	}
}

func TestStatePath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	t.Setenv("HOME", tmp)

	got := StatePath("slots.json")
	want := filepath.Join(tmp, "pom", "slots.json")
	if got != want {
		t.Errorf("StatePath() = %q, want %q", got, want)
	}
}

func TestEnsureStateDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	t.Setenv("HOME", tmp)

	dir := EnsureStateDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("EnsureStateDir did not create %q", dir)
	}
}
