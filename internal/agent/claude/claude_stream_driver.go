package claude

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/pomelohq/pomelo/internal/services"
)

func (d *Driver) buildPersistentArgs() []string {
	args := []string{"-p",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--replay-user-messages",
		"--exclude-dynamic-system-prompt-sections",
	}
	if d.effort != "" {
		args = append(args, "--effort", d.effort)
	}
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
	return args
}

func (d *Driver) ensurePersistent() error {
	d.mu.Lock()
	if d.stdin != nil {
		d.mu.Unlock()
		return nil
	}
	d.mu.Unlock()

	bin := ResolveClaudeBin()
	cmd := exec.Command(bin, d.buildPersistentArgs()...)
	cmd.Dir = d.cwd
	cmd.Env = services.EnvWithToolPath()
	log.Printf("persistent launch: bin=%q cwd=%q mcp=%t model=%s session=%t", bin, d.cwd, d.mcpConfig != "", d.model, d.session != "")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	d.mu.Lock()
	d.cmd = cmd
	d.stdin = stdin
	d.mu.Unlock()

	go func() {
		d.parseStream(stdout)
		werr := cmd.Wait()
		d.mu.Lock()
		d.cmd = nil
		d.stdin = nil
		d.mu.Unlock()
		log.Printf("persistent exited: err=%v stderr=%q", werr, lastLines(strings.TrimSpace(stderr.String()), 4))
		if werr != nil && !isSignalKill(werr) {
			if msg := strings.TrimSpace(stderr.String()); msg != "" {
				d.emit(StreamEvent{Kind: "error", Err: lastLines(msg, 6)})
			}
		}
		select {
		case d.turnEnd <- struct{}{}:
		default:
		}
	}()
	return nil
}

func (d *Driver) sendTurnPersistent(prompt string) {
	if imgs := imageCachePaths(prompt); len(imgs) > 0 {
		prompt += "\n\n[Attached image" + plural(len(imgs)) + " — open each with the Read tool to view: " + strings.Join(imgs, ", ") + "]"
	}
	if err := d.ensurePersistent(); err != nil {
		d.emit(StreamEvent{Kind: "error", Err: err.Error()})
		return
	}
	select {
	case <-d.turnEnd:
	default:
	}
	if err := d.writeUserMessage(prompt); err != nil {
		d.emit(StreamEvent{Kind: "error", Err: err.Error()})
		return
	}
	<-d.turnEnd
}

func (d *Driver) writeUserMessage(text string) error {
	b, _ := json.Marshal(map[string]any{
		"type":    "user",
		"message": map[string]any{"role": "user", "content": text},
	})
	d.mu.Lock()
	in := d.stdin
	d.mu.Unlock()
	if in == nil {
		return fmt.Errorf("claude process not running")
	}
	_, err := in.Write(append(b, '\n'))
	return err
}
