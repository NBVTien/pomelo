package core

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pomelohq/pomelo/internal/jira"
	"github.com/pomelohq/pomelo/internal/provider/forge"
)

func (s *Server) FetchImage(rawURL string) ([]byte, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, "", fmt.Errorf("bad image url")
	}
	if jc := jira.Resolve(s.cfg()); jc != nil && jc.Host() != "" && u.Host == jc.Host() {
		return jc.FetchURL(rawURL)
	}
	req, _ := http.NewRequest(http.MethodGet, rawURL, nil)
	if isGitHubHost(u.Host) {
		if tok := ghToken(); tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("image: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 25<<20))
	if err != nil {
		return nil, "", err
	}
	return data, resp.Header.Get("Content-Type"), nil
}

func (s *Server) FetchImageB64(rawURL string) map[string]any {
	data, ct, err := s.FetchImage(rawURL)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	if ct == "" {
		ct = "image/png"
	}
	return map[string]any{"ok": true, "content_type": ct, "b64": base64.StdEncoding.EncodeToString(data)}
}

func isGitHubHost(host string) bool {
	return host == "github.com" || strings.HasSuffix(host, ".githubusercontent.com") ||
		strings.HasSuffix(host, ".github.com")
}

func ghToken() string { return forge.GithubToken() }
