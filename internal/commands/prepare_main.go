package commands

import (
	"bytes"
	"fmt"
	"github.com/pomelohq/pomelo/internal/provider/shell"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pomelohq/pomelo/internal/config"
	"github.com/pomelohq/pomelo/internal/services"
)

type lineEmitter struct {
	stage string
	emit  func(PrepareEvent)
	mu    sync.Mutex
	buf   bytes.Buffer
}

func (l *lineEmitter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf.Write(p)
	for {
		line, err := l.buf.ReadString('\n')
		if err != nil {
			l.buf.WriteString(line)
			break
		}
		if s := strings.TrimRight(line, "\r\n"); s != "" {
			l.emit(PrepareEvent{Stage: l.stage, Status: "log", Detail: s})
		}
	}
	return len(p), nil
}

func (l *lineEmitter) flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if s := strings.TrimRight(l.buf.String(), "\r\n"); s != "" {
		l.emit(PrepareEvent{Stage: l.stage, Status: "log", Detail: s})
	}
	l.buf.Reset()
}

type PrepareEvent struct {
	Stage  string `json:"stage"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	MS     int64  `json:"ms"`
}

type PrepareOpts struct {
	SkipSeed bool
	Emit     func(PrepareEvent)
}

func PrepareMain(cfg *config.Config, configDir string) error {
	return PrepareMainOpts(cfg, configDir, PrepareOpts{})
}

func PrepareMainOpts(cfg *config.Config, configDir string, opts PrepareOpts) error {
	emit := opts.Emit
	if emit == nil {
		emit = func(PrepareEvent) {}
	}
	branch := cfg.DefaultBranch
	if branch == "" {
		branch = "main"
	}
	wsRoot := filepath.Join(configDir, "workspace--"+branch)
	if !services.DirExists(wsRoot) {
		wsRoot = configDir
	}

	fmt.Printf("%s== Prepare main (%s) ==%s\n\n", Bold, branch, NC)

	services.RegenerateWorkspaceEnv(configDir, cfg, branch)

	run := func(stage string, cmds []string, dir, label string) bool {
		if len(cmds) == 0 {
			return true
		}
		out := cmds
		fmt.Printf("\n%s>>>%s %s %s(%s)%s\n", Blue, NC, label, Dim, dir, NC)
		emit(PrepareEvent{Stage: stage, Status: "log", Detail: "$ " + label})
		lw := &lineEmitter{stage: stage, emit: emit}
		login := shell.Login(strings.Join(out, " && "))
		c := exec.Command(login[0], login[1:]...)
		c.Dir = dir
		c.Stdout = io.MultiWriter(os.Stdout, lw)
		c.Stderr = io.MultiWriter(os.Stderr, lw)
		err := c.Run()
		lw.flush()
		if err != nil {
			fmt.Printf("%s  %s failed (non-fatal): %v%s\n", Yellow, label, err, NC)
			emit(PrepareEvent{Stage: stage, Status: "log", Detail: label + " failed: " + err.Error()})
			return false
		}
		return true
	}

	forEachRepo := func(stage string, cmds func(*config.Dir) []string, verb string) bool {
		ok := true
		for _, name := range cfg.RepoOrder {
			dir := cfg.Repos[name]
			if dir == nil {
				continue
			}
			repoPath := filepath.Join(wsRoot, name)
			if !services.DirExists(repoPath) {
				continue
			}
			if !run(stage, cmds(dir), repoPath, name+": "+verb) {
				ok = false
			}
		}
		return ok
	}

	phases := cfg.PrepareMainPhases()
	emit(PrepareEvent{Stage: "__plan__", Status: "plan", Detail: strings.Join(phases, ",")})

	for i, ph := range phases {
		fmt.Printf("\n%s[%d/%d] %s%s\n", Bold, i+1, len(phases), ph, NC)
		t0 := time.Now()
		switch ph {
		case "reset":
			emit(PrepareEvent{Stage: "reset", Status: "running"})
			if err := DBReset(cfg, configDir, branch); err != nil {
				emit(PrepareEvent{Stage: "reset", Status: "failed", Detail: err.Error(), MS: time.Since(t0).Milliseconds()})
				return fmt.Errorf("db reset: %w", err)
			}
			emit(PrepareEvent{Stage: "reset", Status: "ok", MS: time.Since(t0).Milliseconds()})
		case "migrate":
			emit(PrepareEvent{Stage: "migrate", Status: "running"})
			ok := forEachRepo("migrate", func(d *config.Dir) []string { return d.EffectiveMigrate() }, "migrate")
			emit(PrepareEvent{Stage: "migrate", Status: statusOf(ok), MS: time.Since(t0).Milliseconds()})
		case "seed":
			if opts.SkipSeed {
				fmt.Printf("%s  skipped%s\n", Yellow, NC)
				emit(PrepareEvent{Stage: "seed", Status: "skipped"})
				continue
			}
			emit(PrepareEvent{Stage: "seed", Status: "running"})
			ok := run("seed", cfg.Seed, wsRoot, "workspace: seed")
			if !forEachRepo("seed", func(d *config.Dir) []string { return d.Seed }, "seed") {
				ok = false
			}
			emit(PrepareEvent{Stage: "seed", Status: statusOf(ok), MS: time.Since(t0).Milliseconds()})
		}
	}

	fmt.Printf("\n%sMain prepared.%s New workspaces will clone these DBs via TEMPLATE.\n", Green, NC)
	return nil
}

func statusOf(ok bool) string {
	if ok {
		return "ok"
	}
	return "failed"
}
