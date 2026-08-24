package config

import (
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

func dirOf(path string) string { return filepath.Dir(path) }

func editFile(path string, mutate func(root *yaml.Node)) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	if len(doc.Content) == 0 {
		return nil
	}
	mutate(doc.Content[0])
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	return enc.Close()
}

func fileWithTopKey(files []string, key string) string {
	for _, f := range files {
		if data, err := os.ReadFile(f); err == nil {
			var doc yaml.Node
			if yaml.Unmarshal(data, &doc) == nil && len(doc.Content) > 0 {
				if mapChild(doc.Content[0], key) != nil {
					return f
				}
			}
		}
	}
	return ""
}

func fileWithRepo(files []string, repo string) string {
	for _, f := range files {
		if data, err := os.ReadFile(f); err == nil {
			var doc yaml.Node
			if yaml.Unmarshal(data, &doc) == nil && len(doc.Content) > 0 {
				if repos := mapChild(doc.Content[0], "repos"); repos != nil && mapChild(repos, repo) != nil {
					return f
				}
			}
		}
	}
	return ""
}

func mapChild(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func scalar(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }

func setMapChild(m *yaml.Node, key string, val *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = val
			return
		}
	}
	m.Content = append(m.Content, scalar(key), val)
}

func delMapChild(m *yaml.Node, key string) {
	if m == nil || m.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}

func addPreset(root *yaml.Node, plan *ExtractPlan) {
	presets := mapChild(root, "presets")
	if presets == nil {
		presets = &yaml.Node{Kind: yaml.MappingNode}
		setMapChild(root, "presets", presets)
	}
	p := mapChild(presets, plan.Preset)
	if p == nil {
		p = &yaml.Node{Kind: yaml.MappingNode}
		setMapChild(presets, plan.Preset, p)
	}
	env := mapChild(p, "env")
	if env == nil {
		env = &yaml.Node{Kind: yaml.MappingNode}
		setMapChild(p, "env", env)
	}
	keys := make([]string, 0, len(plan.Keys))
	for k := range plan.Keys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		setMapChild(env, k, scalar(plan.Keys[k]))
	}
}

func editRepoForExtract(root *yaml.Node, repo string, plan *ExtractPlan) {
	repos := mapChild(root, "repos")
	rn := mapChild(repos, repo)
	if rn == nil {
		return
	}
	if env := mapChild(rn, "env"); env != nil {
		targets := []*yaml.Node{env}
		if base := mapChild(env, "*"); base != nil {
			targets = []*yaml.Node{base}
		}
		for _, t := range targets {
			for k := range plan.Keys {
				delMapChild(t, k)
			}
		}
	}
	addPresetRef(rn, plan.Preset)
}

func addPresetRef(repo *yaml.Node, name string) {
	cur := mapChild(repo, "preset")
	switch {
	case cur == nil:
		setMapChild(repo, "preset", &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{scalar(name)}})
	case cur.Kind == yaml.ScalarNode:
		if cur.Value == name {
			return
		}
		setMapChild(repo, "preset", &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{scalar(cur.Value), scalar(name)}})
	case cur.Kind == yaml.SequenceNode:
		for _, c := range cur.Content {
			if c.Value == name {
				return
			}
		}
		cur.Content = append(cur.Content, scalar(name))
	}
}
