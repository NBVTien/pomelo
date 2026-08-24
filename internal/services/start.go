package services

import (
	"fmt"
	"path/filepath"

	"github.com/pomelohq/pomelo/internal/config"
)

func PreflightPort(configDir, wsKey, svcKey string) (int, error) {
	key := svcLocalKey(wsKey, svcKey)
	m := mgr()
	port := m.Acquire(key)
	if port != 0 && !IsPortFree(port) {
		m.Release(key)
		port = m.Acquire(key)
	}
	if port == 0 {
		return 0, fmt.Errorf("no free port available in [%d,%d] — free some and retry", portLo, portHi)
	}
	m.Mark(key, PortStarting)
	return port, nil
}

func BuildServiceCmd(workDir string, dir *config.Dir, svc *config.Service, port int, mode string) string {
	svcDir := workDir
	if svc.Dir != "" {
		svcDir = filepath.Join(workDir, svc.Dir)
	}
	cmd := fmt.Sprintf("cd '%s'", svcDir)
	if svc.PreStart != "" {
		cmd += " && " + svc.PreStart
	} else if dir.PreStart != "" {
		cmd += " && " + dir.PreStart
	}
	if port > 0 {
		cmd += fmt.Sprintf(" && export PORT=%d BIND_IP=%s", port, BindIP())
	}
	cmd += " && " + svc.ActiveCmd(mode)
	if svc.ShellEnv != "" {
		cmd = svc.ShellEnv + " " + cmd
	} else if dir.ShellEnv != "" {
		cmd = dir.ShellEnv + " " + cmd
	}
	return cmd
}
