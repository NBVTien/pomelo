package main

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func firstSubcommand(args []string) (string, int) {
	for i, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a, i
	}
	return "", -1
}

func tryPluginDispatch() {
	args := os.Args[1:]

	name, idx := firstSubcommand(args)
	if name == "" {
		return
	}

	if c, _, err := rootCmd.Find(args); err == nil && c != rootCmd {
		return
	}

	bin, err := exec.LookPath("pom-" + name)
	if err != nil {
		return
	}

	argv := append([]string{bin}, args[idx+1:]...)
	_ = syscall.Exec(bin, argv, os.Environ())
}
