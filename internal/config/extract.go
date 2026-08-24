package config

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

type ExtractPlan struct {
	Preset string
	Keys   map[string]string
	Repos  []string
}

func PlanExtractPreset(path, preset string, keys []string) (*ExtractPlan, error) {
	data, _, err := MergedYAML(path)
	if err != nil {
		return nil, err
	}
	var raw Config
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	envOf := map[string]map[string]string{}
	for name, dir := range raw.Repos {
		if dir == nil {
			continue
		}
		base, _ := parseEnv(&dir.RawEnv)
		envOf[name] = base
	}

	canon := map[string]string{}
	for name, env := range envOf {
		ok := true
		for _, k := range keys {
			if _, has := env[k]; !has {
				ok = false
				break
			}
		}
		if ok {
			for _, k := range keys {
				canon[k] = env[k]
			}
			_ = name
			break
		}
	}
	if len(canon) != len(keys) {
		return nil, fmt.Errorf("no repo has all of %v", keys)
	}

	var repos []string
	for name, env := range envOf {
		match := true
		for _, k := range keys {
			if env[k] != canon[k] {
				match = false
				break
			}
		}
		if match {
			repos = append(repos, name)
		}
	}
	if len(repos) < 2 {
		return nil, fmt.Errorf("only %d repo(s) share those keys identically — need ≥2 to hoist", len(repos))
	}
	sort.Strings(repos)
	return &ExtractPlan{Preset: preset, Keys: canon, Repos: repos}, nil
}

func ApplyExtractPreset(path string, plan *ExtractPlan) error {
	configDir := dirOf(path)
	files := append([]string{path}, FragmentPaths(configDir)...)

	before, err := resolvedEnvs(path, plan.Repos)
	if err != nil {
		return err
	}

	backups := map[string][]byte{}
	restore := func() {
		for f, b := range backups {
			_ = os.WriteFile(f, b, 0o644)
		}
	}
	backup := func(f string) error {
		if _, done := backups[f]; done {
			return nil
		}
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		backups[f] = b
		return nil
	}

	presetFile := fileWithTopKey(files, "presets")
	if presetFile == "" {
		presetFile = path
	}
	if err := backup(presetFile); err != nil {
		return err
	}
	if err := editFile(presetFile, func(root *yaml.Node) { addPreset(root, plan) }); err != nil {
		restore()
		return err
	}

	for _, repo := range plan.Repos {
		f := fileWithRepo(files, repo)
		if f == "" {
			restore()
			return fmt.Errorf("repo %q not found in any config file", repo)
		}
		if err := backup(f); err != nil {
			restore()
			return err
		}
		if err := editFile(f, func(root *yaml.Node) { editRepoForExtract(root, repo, plan) }); err != nil {
			restore()
			return err
		}
	}

	after, err := resolvedEnvs(path, plan.Repos)
	if err != nil {
		restore()
		return err
	}
	for _, repo := range plan.Repos {
		if !envEqual(before[repo], after[repo]) {
			restore()
			return fmt.Errorf("resolved env for %q changed — rolled back (not a safe extract)", repo)
		}
	}
	return nil
}

func resolvedEnvs(path string, repos []string) (map[string]map[string]string, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	out := map[string]map[string]string{}
	for _, r := range repos {
		if dir := cfg.Repos[r]; dir != nil {
			out[r] = dir.Env
		}
	}
	return out, nil
}

func envEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
