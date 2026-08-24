package forge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const githubTokenSecret = "github"

var tokenResolver func() string

var ghHTTP = &http.Client{Timeout: 30 * time.Second}

// GitHub rate limit (5000 GraphQL points/hr + a secondary abuse limit). On 403/429
// we honour the server's reset time and refuse further calls until then, so we
// never hammer a throttled API.
var (
	rlMu      sync.Mutex
	rlRetryAt time.Time
)

func rlBlocked() (time.Time, bool) {
	rlMu.Lock()
	defer rlMu.Unlock()
	return rlRetryAt, time.Now().Before(rlRetryAt)
}

// rlNote records the server-advised wait from a throttled response's headers.
func rlNote(h http.Header) {
	reset := time.Now().Add(60 * time.Second) // conservative default
	if ra := h.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil {
			reset = time.Now().Add(time.Duration(secs) * time.Second)
		}
	} else if h.Get("X-RateLimit-Remaining") == "0" {
		if ts, err := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			reset = time.Unix(ts, 0)
		}
	}
	rlMu.Lock()
	rlRetryAt = reset
	rlMu.Unlock()
}

// GithubToken is the resolved GitHub token (env or per-project secret), shared
// with core (image auth, integrations status).
func GithubToken() string { return ghToken() }

// GithubTest validates a token against GET /user (the passed one, else the
// configured one) — for the Integrations "Test" button.
func GithubTest(token string) map[string]any {
	token = strings.TrimSpace(token)
	if token == "" {
		token = ghToken()
	}
	if token == "" {
		return map[string]any{"ok": false, "error": "no token — paste a PAT or set GH_TOKEN"}
	}
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "pomelo")
	resp, err := ghHTTP.Do(req)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return map[string]any{"ok": false, "error": "token rejected — check it has Pull requests: read"}
	}
	if resp.StatusCode >= 400 {
		return map[string]any{"ok": false, "error": fmt.Sprintf("github %d", resp.StatusCode)}
	}
	var u struct {
		Login string `json:"login"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&u)
	return map[string]any{"ok": true, "user": u.Login}
}

func ghToken() string {
	if t := os.Getenv("GH_TOKEN"); t != "" {
		return t
	}
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	if tokenResolver != nil {
		return tokenResolver()
	}
	return ""
}

func ghDo(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	tok := ghToken()
	if tok == "" {
		return nil, fmt.Errorf("no GitHub token (set GH_TOKEN or add a github token in Integrations)")
	}
	if until, blocked := rlBlocked(); blocked {
		return nil, fmt.Errorf("github rate-limited until %s", until.Format(time.Kitchen))
	}
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "pomelo")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := ghHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		rlNote(resp.Header)
	}
	if resp.StatusCode >= 400 {
		return out, fmt.Errorf("github %s: %d", url, resp.StatusCode)
	}
	return out, nil
}

// gqlQuery POSTs a GraphQL query and returns the raw {data,errors} envelope.
func gqlQuery(ctx context.Context, query string) ([]byte, error) {
	body, _ := json.Marshal(map[string]string{"query": query})
	return ghDo(ctx, http.MethodPost, "https://api.github.com/graphql", body)
}

func restGET(ctx context.Context, path string) ([]byte, error) {
	return ghDo(ctx, http.MethodGet, "https://api.github.com/"+path, nil)
}
