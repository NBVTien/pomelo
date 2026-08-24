package paths

import (
	"os"
	"path/filepath"
)

func xdgBase() string {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return x
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		return "/tmp"
	}
	return filepath.Join(home, ".local", "state")
}

func StateDir() string {
	return filepath.Join(xdgBase(), "pom")
}

func StatePath(rel string) string {
	return filepath.Join(StateDir(), rel)
}

func EnsureStateDir() string {
	dir := StateDir()
	_ = os.MkdirAll(dir, 0o755)
	return dir
}
