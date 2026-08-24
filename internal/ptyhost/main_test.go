package ptyhost

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	xdg, err := os.MkdirTemp("", "pom-pty-xdg")
	if err != nil {
		panic(err)
	}
	sock := fmt.Sprintf("/tmp/pt-%d", os.Getpid())
	os.Setenv("XDG_STATE_HOME", xdg)
	os.Setenv("POM_PTY_SOCK_DIR", sock)
	code := m.Run()
	_ = os.RemoveAll(xdg)
	_ = os.RemoveAll(sock)
	os.Exit(code)
}
