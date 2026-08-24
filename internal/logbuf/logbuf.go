package logbuf

import (
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/pomelohq/pomelo/internal/paths"
)

const (
	maxLines    = 2000
	maxFileSize = 2 << 20
)

type ring struct {
	mu    sync.Mutex
	lines []string
	file  *os.File
}

var Default = &ring{lines: make([]string, 0, maxLines)}

func (r *ring) Write(p []byte) (int, error) {
	r.mu.Lock()
	r.lines = append(r.lines, strings.Split(strings.TrimRight(string(p), "\n"), "\n")...)
	if len(r.lines) > maxLines {
		r.lines = append(r.lines[:0], r.lines[len(r.lines)-maxLines:]...)
	}
	f := r.file
	r.mu.Unlock()
	if f != nil {
		_, _ = f.Write(p)
	}
	return len(p), nil
}

func Lines() []string {
	Default.mu.Lock()
	defer Default.mu.Unlock()
	return append([]string(nil), Default.lines...)
}

func FilePath() string { return paths.StatePath("app.log") }

func Setup() {
	path := FilePath()
	if fi, err := os.Stat(path); err == nil && fi.Size() > maxFileSize {
		_ = os.Truncate(path, 0)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	Default.mu.Lock()
	if err == nil {
		Default.file = f
	}
	Default.mu.Unlock()
	log.SetOutput(io.MultiWriter(os.Stderr, Default))
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
}
