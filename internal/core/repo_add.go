package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pomelohq/pomelo/internal/config"
	"gopkg.in/yaml.v3"
)

func (s *Server) handleRepoClone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if s.cfg() == nil || s.WorkspaceRoot == "" {
		http.Error(w, "no project loaded", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		URL      string `json:"url"`
		Alias    string `json:"alias"`
		EnvFiles []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"env_files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	url := strings.TrimSpace(req.URL)
	if url == "" || strings.HasPrefix(url, "-") || !isGitURL(url) {
		http.Error(w, "provide a git URL (SSH or HTTPS)", http.StatusBadRequest)
		return
	}
	repoName := repoNameFromURL(url)
	if repoName == "" {
		http.Error(w, "cannot derive repo name from URL", http.StatusBadRequest)
		return
	}
	if _, exists := s.cfg().Repos[repoName]; exists {
		http.Error(w, "repo already in this project: "+repoName, http.StatusConflict)
		return
	}

	mainDir := s.workspaceRoot(s.DefaultBranch, true)
	dst := filepath.Join(mainDir, repoName)
	if _, err := osStat(dst); err == nil {
		http.Error(w, "a directory already exists at "+dst, http.StatusConflict)
		return
	}
	if err := cloneRemote(url, dst); err != nil {
		http.Error(w, "clone: "+err.Error(), http.StatusInternalServerError)
		return
	}

	for _, ef := range req.EnvFiles {
		rel := strings.TrimSpace(ef.Path)
		if rel == "" || filepath.IsAbs(rel) || strings.Contains(rel, "..") {
			continue
		}
		p := filepath.Join(dst, filepath.Clean(rel))
		if p != dst && !strings.HasPrefix(p, dst+string(os.PathSeparator)) {
			continue
		}
		if os.MkdirAll(filepath.Dir(p), 0o755) == nil {
			_ = os.WriteFile(p, []byte(ef.Content), 0o644)
		}
	}

	if err := addRepoToConfigYAML(s.configPath(), repoName, strings.TrimSpace(req.Alias)); err != nil {
		http.Error(w, "update pom.yml: "+err.Error(), http.StatusInternalServerError)
		return
	}
	cfg, err := config.Load(s.configPath())
	if err != nil {
		http.Error(w, "config reload: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.setCfg(cfg)
	s.Project = cfg.Session
	s.DefaultBranch = cfg.GlobalDefaultBranch()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "repo": repoName})
}

func addRepoToConfigYAML(path, repoName, alias string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("unexpected config shape")
	}
	root := doc.Content[0]

	var repos *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "repos" {
			repos = root.Content[i+1]
			break
		}
	}
	if repos == nil {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "repos"},
			&yaml.Node{Kind: yaml.MappingNode})
		repos = root.Content[len(root.Content)-1]
	}
	if repos.Kind != yaml.MappingNode {
		return fmt.Errorf("`repos` is not a mapping")
	}

	val := &yaml.Node{Kind: yaml.MappingNode}
	if alias != "" {
		val.Content = append(val.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "alias"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: alias})
	} else {
		val.Style = yaml.FlowStyle
	}
	repos.Content = append(repos.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: repoName}, val)

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
