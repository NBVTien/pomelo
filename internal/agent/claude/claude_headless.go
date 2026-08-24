package claude

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"github.com/pomelohq/pomelo/internal/services"
)

var regexpImageCache = regexp.MustCompile(`/\S*\.claude/image-cache/\S+?\.(?:png|jpe?g|gif|webp)`)

const autoCompactTokens = 700_000

func pct(tokens int) int {
	p := tokens * 100 / 1_000_000
	if p > 100 {
		p = 100
	}
	return p
}

type StreamEvent struct {
	Kind       string          `json:"kind"`
	Text       string          `json:"text,omitempty"`
	Tool       string          `json:"tool,omitempty"`
	ToolID     string          `json:"tool_id,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
	Result     string          `json:"result,omitempty"`
	Session    string          `json:"session,omitempty"`
	Model      string          `json:"model,omitempty"`
	Cost       float64         `json:"cost,omitempty"`
	Ctx        int             `json:"ctx,omitempty"`
	CacheRead  int             `json:"cache_read,omitempty"`
	CacheWrite int             `json:"cache_write,omitempty"`
	InTok      int             `json:"in_tok,omitempty"`
	OutTok     int             `json:"out_tok,omitempty"`
	RateType   string          `json:"rate_type,omitempty"`
	ResetsAt   int64           `json:"resets_at,omitempty"`
	RateStatus string          `json:"rate_status,omitempty"`
	Queue      []string        `json:"queue,omitempty"`
	Busy       bool            `json:"busy,omitempty"`
	Err        string          `json:"error,omitempty"`
}

type Driver struct {
	cwd    string
	mode   string
	model  string
	effort string

	mcpConfig string
	sysPrompt string
	addDirs   []string
	allowed   string

	notify func(running bool)

	mu          sync.Mutex
	session     string
	queue       []string
	running     bool
	cmd         *exec.Cmd
	curText     string
	wantCompact bool
	authIn      io.WriteCloser
	authCmd     *exec.Cmd
	subs        map[chan []byte]struct{}

	persistent bool
	stdin      io.WriteCloser
	turnEnd    chan struct{}
}

func newHeadlessDriver(cwd, mode, model string) *Driver {
	return &Driver{cwd: cwd, mode: mode, model: model,
		subs: map[chan []byte]struct{}{}, turnEnd: make(chan struct{}, 1)}
}

func deterministicSessionID(key string) string {
	h := sha1.Sum([]byte("pom-session:" + key))
	b := h[:16]
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func imageCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "image-cache")
}

func sessionFilePath(cwd, id string) string {
	return filepath.Join(claudeProjectsRoot(), encodeClaudeProjectDir(cwd), id+".jsonl")
}

func sessionExists(cwd, id string) bool {
	if id == "" {
		return false
	}
	_, err := os.Stat(sessionFilePath(cwd, id))
	return err == nil
}

func (d *Driver) appendCurText(s string) { d.mu.Lock(); d.curText += s; d.mu.Unlock() }
func (d *Driver) setCurText(s string)    { d.mu.Lock(); d.curText = s; d.mu.Unlock() }

func (d *Driver) Subscribe() chan []byte {
	ch := make(chan []byte, 256)
	d.mu.Lock()
	d.subs[ch] = struct{}{}
	q := append([]string(nil), d.queue...)
	running := d.running
	curText := d.curText
	d.mu.Unlock()
	ch <- mustJSON(StreamEvent{Kind: "queue", Queue: q})
	ch <- mustJSON(StreamEvent{Kind: "busy", Busy: running})
	if running && curText != "" {
		ch <- mustJSON(StreamEvent{Kind: "text", Text: curText})
	}
	return ch
}

func (d *Driver) Unsubscribe(ch chan []byte) {
	d.mu.Lock()
	delete(d.subs, ch)
	d.mu.Unlock()
}

func (d *Driver) emit(ev StreamEvent) {
	b := mustJSON(ev)
	d.mu.Lock()
	for ch := range d.subs {
		select {
		case ch <- b:
		default:
		}
	}
	d.mu.Unlock()
}

func (d *Driver) broadcastQueue() {
	d.mu.Lock()
	q := append([]string(nil), d.queue...)
	d.mu.Unlock()
	d.emit(StreamEvent{Kind: "queue", Queue: q})
}

func (d *Driver) Enqueue(text string) {
	d.mu.Lock()
	d.queue = append(d.queue, text)
	start := !d.running
	d.mu.Unlock()
	d.broadcastQueue()
	if start {
		d.runNext()
	}
}

func (d *Driver) clearSession() {
	d.mu.Lock()
	if d.session != "" {
		_ = os.Remove(sessionFilePath(d.cwd, d.session))
	}
	d.queue = nil
	d.curText = ""
	cmd := d.cmd
	if d.persistent {
		d.stdin = nil
	}
	d.mu.Unlock()
	if d.persistent && cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	d.emit(StreamEvent{Kind: "cleared"})
	d.broadcastQueue()
}

func (d *Driver) handleSlash(t string) bool {
	head := t
	if i := strings.IndexByte(t, ' '); i >= 0 {
		head = t[:i]
	}
	switch head {
	case "/clear", "/new":
		d.clearSession()
		return true
	case "/login":
		args := []string{"--claudeai"}
		if rest := strings.TrimSpace(t[len(head):]); rest != "" {
			args = strings.Fields(rest)
		}
		go d.startLogin(args...)
		return true
	case "/logout":
		go d.runLogout()
		return true
	case "/whoami", "/status":
		go func() { d.emit(StreamEvent{Kind: "system", Text: authStatusLine(ResolveClaudeBin(), d.cwd)}) }()
		return true
	}
	return false
}

func (d *Driver) feedAuthCode(text string) bool {
	d.mu.Lock()
	in := d.authIn
	d.mu.Unlock()
	if in == nil || strings.TrimSpace(text) == "" {
		return false
	}
	_, _ = io.WriteString(in, strings.TrimSpace(text)+"\n")
	d.emit(StreamEvent{Kind: "system", Text: "Submitting code…"})
	return true
}

func (d *Driver) Send(text string) {
	if d.feedAuthCode(text) {
		return
	}
	t := strings.TrimSpace(text)
	if d.handleSlash(t) {
		return
	}
	if t != "" {
		d.Enqueue(text)
	}
}

func (d *Driver) Busy() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.running || len(d.queue) > 0
}

func (d *Driver) Stop() {
	d.mu.Lock()
	cmd := d.cmd
	authCmd := d.authCmd
	d.queue = nil
	d.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
	if authCmd != nil && authCmd.Process != nil {
		_ = authCmd.Process.Signal(syscall.SIGTERM)
	}
	d.broadcastQueue()
}

func (d *Driver) Cancel(i int) {
	d.mu.Lock()
	if i >= 0 && i < len(d.queue) {
		d.queue = append(d.queue[:i], d.queue[i+1:]...)
	}
	d.mu.Unlock()
	d.broadcastQueue()
}

func (d *Driver) runNext() {
	d.mu.Lock()
	if d.running || len(d.queue) == 0 {
		d.mu.Unlock()
		return
	}
	prompt := d.queue[0]
	d.running = true
	d.mu.Unlock()
	d.emit(StreamEvent{Kind: "busy", Busy: true})
	if d.notify != nil {
		d.notify(true)
	}

	go func() {
		if d.persistent {
			d.sendTurnPersistent(prompt)
		} else {
			d.runTurn(prompt)
		}
		d.mu.Lock()
		if len(d.queue) > 0 {
			d.queue = d.queue[1:]
		}
		if d.wantCompact && strings.TrimSpace(prompt) != "/compact" {
			d.wantCompact = false
			d.queue = append([]string{"/compact"}, d.queue...)
			go d.emit(StreamEvent{Kind: "system", Text: "⤺ Auto-compacting — context is large; summarizing to keep the next prompts cheap"})
		}
		d.running = false
		more := len(d.queue) > 0
		d.mu.Unlock()
		d.emit(StreamEvent{Kind: "busy", Busy: false})
		d.broadcastQueue()
		if !more && d.notify != nil {
			d.notify(false)
		}
		if more {
			d.runNext()
		}
	}()
}

func (d *Driver) runTurn(prompt string) {
	bin := ResolveClaudeBin()
	if imgs := imageCachePaths(prompt); len(imgs) > 0 {
		prompt += "\n\n[Attached image" + plural(len(imgs)) + " — open each with the Read tool to view: " + strings.Join(imgs, ", ") + "]"
	}
	args := []string{"-p", prompt,
		"--output-format", "stream-json", "--verbose", "--include-partial-messages"}
	switch d.mode {
	case "bypassPermissions":
		args = append(args, "--dangerously-skip-permissions")
	case "acceptEdits", "plan":
		args = append(args, "--permission-mode", d.mode)
	}
	if d.model != "" {
		args = append(args, "--model", d.model)
	}
	if d.mcpConfig != "" {
		args = append(args, "--mcp-config", d.mcpConfig, "--strict-mcp-config")
	}
	for _, dir := range d.addDirs {
		if dir != "" {
			args = append(args, "--add-dir", dir)
		}
	}
	if d.sysPrompt != "" {
		args = append(args, "--append-system-prompt", d.sysPrompt)
	}
	if d.allowed != "" {
		args = append(args, "--allowedTools", d.allowed)
	}
	d.mu.Lock()
	sid := d.session
	d.mu.Unlock()
	if sid != "" {
		if sessionExists(d.cwd, sid) {
			args = append(args, "--resume", sid)
		} else {
			args = append(args, "--session-id", sid)
		}
	}

	cmd := exec.Command(bin, args...)
	cmd.Dir = d.cwd
	cmd.Env = services.EnvWithToolPath()
	log.Printf("claude launch: bin=%q cwd=%q mcp=%t model=%s session=%t", bin, d.cwd, d.mcpConfig != "", d.model, sid != "")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		d.emit(StreamEvent{Kind: "error", Err: err.Error()})
		return
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		d.emit(StreamEvent{Kind: "error", Err: err.Error()})
		return
	}
	d.mu.Lock()
	d.cmd = cmd
	d.mu.Unlock()

	d.parseStream(stdout)
	werr := cmd.Wait()
	log.Printf("claude exited: err=%v stderr=%q", werr, lastLines(strings.TrimSpace(stderr.String()), 4))
	if werr != nil && !isSignalKill(werr) {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			d.emit(StreamEvent{Kind: "error", Err: lastLines(msg, 6)})
		}
	}
	d.mu.Lock()
	d.cmd = nil
	d.mu.Unlock()
}

func imageCachePaths(s string) []string {
	re := regexpImageCache
	m := re.FindAllString(s, -1)
	seen := map[string]bool{}
	var out []string
	for _, p := range m {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func isSignalKill(err error) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode() == -1 || ee.ExitCode() == 143
	}
	return false
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

func (s *Feature) DriverFor(branch string, isMain bool, mode, model string) *Driver {
	key := services.WsKey(branch, isMain)
	s.hdMu.Lock()
	defer s.hdMu.Unlock()
	d := s.headless[key]
	if d == nil {
		d = newHeadlessDriver(s.workspaceRoot(branch, isMain), mode, model)
		s.writeWorkspaceMap(branch, isMain)
		d.session = deterministicSessionID("chat:" + key)
		d.mcpConfig = s.mcpConfigJSON(branch)
		d.sysPrompt = s.chatSystemPrompt(branch)
		d.addDirs = append(d.addDirs, imageCacheDir())
		d.allowed = "mcp__pom"
		svc := s.claudeServiceName()
		d.notify = func(running bool) {
			state := "idle"
			prev := "thinking"
			if running {
				state, prev = "thinking", "idle"
			}
			s.broadcastAgent(agentEvent{Branch: branch, IsMain: isMain, Service: svc, State: state, Prev: prev})
		}
		s.headless[key] = d
	} else {
		d.mode, d.model = mode, model
	}
	return d
}

func (s *Feature) FixerDriver(branch string, isMain bool, model string) *Driver {
	return s.agentDriver("fixer", fixerSystemPrompt, branch, isMain, model)
}

func (s *Feature) OnboarderDriver(branch string, isMain bool, model string) *Driver {
	return s.agentDriver("onboarder", onboardSystemPrompt, branch, isMain, model)
}

func (s *Feature) agentDriver(role string, sysPrompt func(string) string, branch string, isMain bool, model string) *Driver {
	if model == "" {
		model = "sonnet"
	}
	cwd := s.WorkspaceRoot
	mcpBranch := s.cfg().GlobalDefaultBranch()
	key := role + ":project"
	addDirs := []string{imageCacheDir()}
	if branch != "" {
		mcpBranch = branch
		key = role + ":" + services.WsKey(branch, isMain)
		addDirs = append(addDirs, s.workspaceRoot(branch, isMain))
	}
	s.hdMu.Lock()
	defer s.hdMu.Unlock()
	if d := s.headless[key]; d != nil {
		d.model = model
		return d
	}
	d := newHeadlessDriver(cwd, "bypassPermissions", model)
	d.persistent = true
	d.session = deterministicSessionID(key)
	d.mcpConfig = s.mcpConfigJSON(mcpBranch)
	d.addDirs = addDirs
	d.sysPrompt = sysPrompt(branch)
	d.allowed = "mcp__pom Read Grep Glob Edit Write MultiEdit Bash"
	if branch != "" {
		svc := s.claudeServiceName()
		d.notify = func(running bool) {
			state, prev := "idle", "thinking"
			if running {
				state, prev = "thinking", "idle"
			}
			s.broadcastAgent(agentEvent{Branch: branch, IsMain: isMain, Service: svc, State: state, Prev: prev})
		}
	}
	s.headless[key] = d
	return d
}

const EnvIsGeneratedNote = "IMPORTANT: `.env.local` (and any per-workspace env file) is GENERATED by Pomelo from the " +
	"`env:` section of the config (`pom.yml` / `pom.d`) with the templating resolved for this branch — editing it " +
	"directly is WRONG and gets overwritten on the next generate/reload. To change an env value, edit the `env:` in " +
	"the Pomelo config via the config tools (config_files → config_file_get/config_file_set for split pom.d, else " +
	"config_set), then config_validate + reload so pom regenerates the env file. Never hand-edit `.env.local`."
