package core

import (
	"os"
	"strings"
)

func osStat(path string) (os.FileInfo, error) { return os.Stat(path) }

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
