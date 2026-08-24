package workspace

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Repo struct {
	Name string
	Path string
}

type WS struct {
	Branch string
	IsMain bool
	Path   string
	Repos  []Repo
}

func IsGitRepo(path string) bool {
	st, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return st.IsDir() || st.Mode().IsRegular()
}

func Scan(root, defaultBranch string, known map[string]bool) []WS {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	mainWs := WS{Branch: defaultBranch, IsMain: true, Path: root}
	branchWs := map[string]*WS{}

	addRepo := func(ws *WS, name, path string) {
		ws.Repos = append(ws.Repos, Repo{Name: name, Path: path})
	}

	migratedMain := false
	for _, e := range entries {
		if e.IsDir() && e.Name() == "workspace--"+defaultBranch {
			migratedMain = true
			break
		}
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		full := filepath.Join(root, name)
		if branch, ok := strings.CutPrefix(name, "workspace--"); ok {
			var target *WS
			if branch == defaultBranch {
				mainWs.Path = full
				target = &mainWs
			} else {
				target = &WS{Branch: branch, IsMain: false, Path: full}
				branchWs[branch] = target
			}
			repoEntries, _ := os.ReadDir(full)
			for _, re := range repoEntries {
				if !re.IsDir() {
					continue
				}
				repoPath := filepath.Join(full, re.Name())
				if !IsGitRepo(repoPath) {
					continue
				}
				addRepo(target, re.Name(), repoPath)
			}
			continue
		}
		if migratedMain {
			continue
		}
		if known != nil && !known[name] {
			continue
		}
		if IsGitRepo(full) {
			addRepo(&mainWs, name, full)
		}
	}

	var out []WS
	if len(mainWs.Repos) > 0 {
		sort.Slice(mainWs.Repos, func(i, j int) bool { return mainWs.Repos[i].Name < mainWs.Repos[j].Name })
		out = append(out, mainWs)
	}
	var keys []string
	for k := range branchWs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ws := branchWs[k]
		sort.Slice(ws.Repos, func(i, j int) bool { return ws.Repos[i].Name < ws.Repos[j].Name })
		out = append(out, *ws)
	}
	return out
}
