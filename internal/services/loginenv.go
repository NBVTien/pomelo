package services

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
)

var loginEnvOnce sync.Once

var (
	toolPathOnce sync.Once
	toolPath     string
)

// ToolPath is the current PATH with well-known version-manager bin/shim dirs
// prepended (nvm default, fnm/volta/asdf/rbenv/pyenv, bun, pnpm, homebrew, …).
// Those managers init only in interactive .zshrc, so tools we spawn — and the
// hooks they run under /bin/sh — otherwise can't find node etc.
//
// Deterministic on purpose: we read the managers' own dirs directly and NEVER
// source .zshrc, so it cannot trigger a macOS TCC/permission prompt from a
// prompt-plugin. Computed once, lazily.
func ToolPath() string {
	toolPathOnce.Do(func() { toolPath = buildToolPath() })
	return toolPath
}

func buildToolPath() string {
	home, _ := os.UserHomeDir()
	base := os.Getenv("PATH")
	have := map[string]bool{}
	for _, d := range filepath.SplitList(base) {
		have[d] = true
	}

	var prepend []string
	add := func(dir string) {
		if dir == "" || have[dir] {
			return
		}
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			prepend = append(prepend, dir)
			have[dir] = true
		}
	}

	if h := os.Getenv("PNPM_HOME"); h != "" {
		add(h)
	}
	add(filepath.Join(home, "Library", "pnpm"))
	add(filepath.Join(envOr("VOLTA_HOME", filepath.Join(home, ".volta")), "bin"))
	add(filepath.Join(home, ".bun", "bin"))
	add(filepath.Join(envOr("XDG_DATA_HOME", filepath.Join(home, ".local", "share")), "fnm"))
	add(filepath.Join(envOr("ASDF_DATA_DIR", filepath.Join(home, ".asdf")), "shims"))
	add(filepath.Join(envOr("RBENV_ROOT", filepath.Join(home, ".rbenv")), "shims"))
	add(filepath.Join(envOr("PYENV_ROOT", filepath.Join(home, ".pyenv")), "shims"))
	add(nvmDefaultBin(home))
	add(filepath.Join(home, ".cargo", "bin"))
	add("/opt/homebrew/bin")
	add("/usr/local/bin")

	if len(prepend) == 0 {
		return base
	}
	return strings.Join(prepend, string(filepath.ListSeparator)) + string(filepath.ListSeparator) + base
}

// nvmDefaultBin resolves nvm's default node bin dir without sourcing anything:
// the `default` alias if it names an installed version, else the highest one.
func nvmDefaultBin(home string) string {
	dir := envOr("NVM_DIR", filepath.Join(home, ".nvm"))
	versionsDir := filepath.Join(dir, "versions", "node")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return ""
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "v") {
			versions = append(versions, e.Name())
		}
	}
	if len(versions) == 0 {
		return ""
	}
	if alias, err := os.ReadFile(filepath.Join(dir, "alias", "default")); err == nil {
		want := strings.TrimSpace(string(alias))
		if !strings.HasPrefix(want, "v") {
			want = "v" + want
		}
		for _, v := range versions {
			if v == want {
				return filepath.Join(versionsDir, v, "bin")
			}
		}
	}
	sort.Slice(versions, func(i, j int) bool { return semverLess(versions[j], versions[i]) })
	return filepath.Join(versionsDir, versions[0], "bin")
}

func semverLess(a, b string) bool {
	pa, pb := parseSemver(a), parseSemver(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

func parseSemver(v string) [3]int {
	var out [3]int
	for i, p := range strings.SplitN(strings.TrimPrefix(v, "v"), ".", 3) {
		n := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
		}
		out[i] = n
	}
	return out
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// EnvWithToolPath returns os.Environ() with PATH replaced by ToolPath().
func EnvWithToolPath() []string {
	env := os.Environ()
	out := env[:0]
	for _, e := range env {
		if !strings.HasPrefix(e, "PATH=") {
			out = append(out, e)
		}
	}
	return append(out, "PATH="+ToolPath())
}

func LoadLoginShellEnv() {
	loginEnvOnce.Do(func() {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "zsh"
		}
		cmd := exec.Command(shell, "-lc", "env")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		cmd.Stdin = nil
		out, err := cmd.Output()
		if err != nil {
			return
		}
		for _, line := range strings.Split(string(out), "\n") {
			i := strings.IndexByte(line, '=')
			if i <= 0 {
				continue
			}
			k := line[:i]
			if k == "PATH" || os.Getenv(k) == "" {
				_ = os.Setenv(k, line[i+1:])
			}
		}
	})
}
