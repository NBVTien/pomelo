package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var lifecycleKeys = []string{"copy", "setup", "seed", "pre_delete", "pre_start", "shortcuts"}

func GroupLifecycle(path string, write bool) (map[string]int, error) {
	data, _, err := MergedYAML(path)
	if err != nil {
		return nil, err
	}
	var raw Config
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	plan := map[string]int{}
	for name := range raw.Repos {
		f := fileWithRepo(append([]string{path}, FragmentPaths(dirOf(path))...), name)
		if f == "" {
			continue
		}
		n := countTopLifecycleKeys(f, name)
		if n > 0 {
			plan[name] = n
		}
	}
	if !write || len(plan) == 0 {
		return plan, nil
	}

	before, err := lifecycleSnapshot(path)
	if err != nil {
		return nil, err
	}
	files := append([]string{path}, FragmentPaths(dirOf(path))...)
	backups := map[string][]byte{}
	restore := func() {
		for f, b := range backups {
			_ = os.WriteFile(f, b, 0o644)
		}
	}
	for name := range plan {
		f := fileWithRepo(files, name)
		if _, done := backups[f]; !done {
			b, err := os.ReadFile(f)
			if err != nil {
				restore()
				return nil, err
			}
			backups[f] = b
		}
		if err := editFile(f, func(root *yaml.Node) { groupRepoLifecycle(root, name) }); err != nil {
			restore()
			return nil, err
		}
	}

	after, err := lifecycleSnapshot(path)
	if err != nil {
		restore()
		return nil, err
	}
	if before != after {
		restore()
		return nil, fmt.Errorf("effective lifecycle changed — rolled back (not a safe move)")
	}
	return plan, nil
}

func countTopLifecycleKeys(file, repo string) int {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0
	}
	var doc yaml.Node
	if yaml.Unmarshal(data, &doc) != nil || len(doc.Content) == 0 {
		return 0
	}
	rn := mapChild(mapChild(doc.Content[0], "repos"), repo)
	if rn == nil {
		return 0
	}
	n := 0
	for _, k := range lifecycleKeys {
		if mapChild(rn, k) != nil {
			n++
		}
	}
	return n
}

func groupRepoLifecycle(root *yaml.Node, repo string) {
	rn := mapChild(mapChild(root, "repos"), repo)
	if rn == nil {
		return
	}
	lc := mapChild(rn, "lifecycle")
	if lc == nil {
		lc = &yaml.Node{Kind: yaml.MappingNode}
	}
	moved := false
	for _, k := range lifecycleKeys {
		if v := mapChild(rn, k); v != nil {
			setMapChild(lc, k, v)
			delMapChild(rn, k)
			moved = true
		}
	}
	if moved {
		setMapChild(rn, "lifecycle", lc)
	}
}

func lifecycleSnapshot(path string) (string, error) {
	cfg, err := Load(path)
	if err != nil {
		return "", err
	}
	var b []byte
	for _, name := range cfg.RepoOrder {
		d := cfg.Repos[name]
		if d == nil {
			continue
		}
		b = append(b, fmt.Sprintf("%s|copy=%v|setup=%v|seed=%v|predel=%v|prestart=%s|sc=%v\n",
			name, d.Copy, d.Setup, d.Seed, d.PreDelete, d.PreStart, d.Shortcuts)...)
	}
	return string(b), nil
}
