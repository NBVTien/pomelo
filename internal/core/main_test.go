package core

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "pom-web-test-state")
	if err != nil {
		panic(err)
	}
	sock := fmt.Sprintf("/tmp/pt-web-%d", os.Getpid())
	os.Setenv("XDG_STATE_HOME", dir)
	os.Setenv("POM_PTY_SOCK_DIR", sock)
	code := m.Run()
	_ = os.RemoveAll(dir)
	_ = os.RemoveAll(sock)
	os.Exit(code)
}
