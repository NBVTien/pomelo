package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var regexpAuthURL = regexp.MustCompile(`https://[^\s\x1b\x07\]]+`)

func (d *Driver) startLogin(extra ...string) {
	d.mu.Lock()
	busy := d.authIn != nil
	d.mu.Unlock()
	if busy {
		d.emit(StreamEvent{Kind: "system", Text: "A sign-in is already in progress — paste the code, or /logout to cancel."})
		return
	}
	bin := ResolveClaudeBin()
	cmd := exec.Command(bin, append([]string{"auth", "login"}, extra...)...)
	cmd.Dir = d.cwd
	in, err := cmd.StdinPipe()
	if err != nil {
		d.emit(StreamEvent{Kind: "error", Err: err.Error()})
		return
	}
	r, wpipe := io.Pipe()
	cmd.Stdout, cmd.Stderr = wpipe, wpipe
	if err := cmd.Start(); err != nil {
		d.emit(StreamEvent{Kind: "error", Err: err.Error()})
		return
	}
	d.mu.Lock()
	d.authIn, d.authCmd = in, cmd
	d.mu.Unlock()
	d.emit(StreamEvent{Kind: "system", Text: "Opening your browser to sign in…"})

	go func() {
		_ = cmd.Wait()
		_ = wpipe.Close()
		d.mu.Lock()
		d.authIn, d.authCmd = nil, nil
		d.mu.Unlock()
		if st := authStatusLine(bin, d.cwd); st != "" {
			d.emit(StreamEvent{Kind: "system", Text: st})
		}
	}()

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 8*1024), 256*1024)
	urlShown, hinted := false, false
	for sc.Scan() {
		ln := stripANSI(sc.Text())
		if !urlShown {
			if u := regexpAuthURL.FindString(ln); u != "" {
				d.emit(StreamEvent{Kind: "system", Text: "If the browser didn't open, sign in here: " + u})
				urlShown = true
			}
		}
		if !hinted && strings.Contains(strings.ToLower(ln), "paste code") {
			d.emit(StreamEvent{Kind: "system", Text: "After approving, paste the code here and press Enter."})
			hinted = true
		}
	}
}

func (d *Driver) runLogout() {
	bin := ResolveClaudeBin()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "auth", "logout")
	cmd.Dir = d.cwd
	_ = cmd.Run()
	d.emit(StreamEvent{Kind: "system", Text: authStatusLine(bin, d.cwd)})
}

func authStatusLine(bin, dir string) string {
	cmd := exec.Command(bin, "auth", "status", "--json")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "Not signed in."
	}
	var st struct {
		LoggedIn         bool   `json:"loggedIn"`
		Email            string `json:"email"`
		AuthMethod       string `json:"authMethod"`
		SubscriptionType string `json:"subscriptionType"`
	}
	if json.Unmarshal(out, &st) != nil || !st.LoggedIn {
		return "Not signed in."
	}
	line := "Signed in as " + st.Email
	if st.SubscriptionType != "" {
		line += " (" + st.SubscriptionType + " · " + st.AuthMethod + ")"
	}
	return line
}

var regexpANSI = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func stripANSI(s string) string { return regexpANSI.ReplaceAllString(s, "") }
