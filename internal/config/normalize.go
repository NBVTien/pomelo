package config

import (
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

var colonToDot = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`\{\{conn:([\w-]+)\}\}`), "{{shared.$1.url}}"},
	{regexp.MustCompile(`\{\{host:([\w-]+)\}\}`), "{{shared.$1.host}}"},
	{regexp.MustCompile(`\{\{port:([\w-]+)\}\}`), "{{shared.$1.port}}"},
	{regexp.MustCompile(`\{\{user:([\w-]+)\}\}`), "{{shared.$1.user}}"},
	{regexp.MustCompile(`\{\{pass:([\w-]+)\}\}`), "{{shared.$1.pass}}"},
	{regexp.MustCompile(`\{\{slot:([\w-]+)\}\}`), "{{shared.$1.slot}}"},
	{regexp.MustCompile(`\{\{db:([\w-]+)\}\}`), "{{db.$1}}"},
	{regexp.MustCompile(`\{\{branch_safe\}\}`), "{{branch.safe}}"},
	{regexp.MustCompile(`\{\{branch_hash\}\}`), "{{branch.hash}}"},
	{regexp.MustCompile(`\{\{branch_host\}\}`), "{{branch.host}}"},
}

func migrateColonTokens(s string) string {
	for _, m := range colonToDot {
		s = m.re.ReplaceAllString(s, m.repl)
	}
	return s
}

var removedTopKeys = []string{"schema_version", "plugins", "combinations", "proxy", "webhook"}

func RemovedKeys(path string) []string {
	data, _, err := MergedYAML(path)
	if err != nil {
		return nil
	}
	return RemovedKeysInYAML(data)
}

func RemovedKeysInYAML(data []byte) []string {
	var doc yaml.Node
	if yaml.Unmarshal(data, &doc) != nil || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	seen := map[string]bool{}
	var out []string
	add := func(k string) {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for _, k := range removedTopKeys {
		if findKey(root, k) >= 0 {
			add(k)
		}
	}
	if ri := findKey(root, "repos"); ri >= 0 {
		if repos := root.Content[ri+1]; repos.Kind == yaml.MappingNode {
			for j := 1; j < len(repos.Content); j += 2 {
				repo := repos.Content[j]
				for _, k := range []string{"plugins", "exposes"} {
					if findKey(repo, k) >= 0 {
						add(k)
					}
				}
				if si := findKey(repo, "services"); si >= 0 {
					if svcs := repo.Content[si+1]; svcs.Kind == yaml.MappingNode {
						for s := 1; s < len(svcs.Content); s += 2 {
							if findKey(svcs.Content[s], "exposes") >= 0 {
								add("exposes")
							}
						}
					}
				}
			}
		}
	}
	return out
}

func Normalize(path string) ([]string, error) {
	seen := map[string]bool{}
	var removed []string
	add := func(k string) {
		if !seen[k] {
			seen[k] = true
			removed = append(removed, k)
		}
	}
	files := append([]string{path}, FragmentPaths(filepath.Dir(path))...)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var doc yaml.Node
		if yaml.Unmarshal(data, &doc) != nil || len(doc.Content) == 0 {
			continue
		}
		root := doc.Content[0]
		changed := false
		for _, k := range removedTopKeys {
			if i := findKey(root, k); i >= 0 {
				removeKey(root, i)
				add(k)
				changed = true
			}
		}
		if ri := findKey(root, "repos"); ri >= 0 {
			if repos := root.Content[ri+1]; repos.Kind == yaml.MappingNode {
				for j := 1; j < len(repos.Content); j += 2 {
					repo := repos.Content[j]
					for _, k := range []string{"plugins", "exposes"} {
						if i := findKey(repo, k); i >= 0 {
							removeKey(repo, i)
							add(k)
							changed = true
						}
					}
					if si := findKey(repo, "services"); si >= 0 {
						if svcs := repo.Content[si+1]; svcs.Kind == yaml.MappingNode {
							for s := 1; s < len(svcs.Content); s += 2 {
								if i := findKey(svcs.Content[s], "exposes"); i >= 0 {
									removeKey(svcs.Content[s], i)
									add("exposes")
									changed = true
								}
							}
						}
					}
				}
			}
		}
		if changed {
			out, err := yaml.Marshal(&doc)
			if err != nil {
				return removed, err
			}
			if err := os.WriteFile(f, out, 0o644); err != nil {
				return removed, err
			}
		}
	}
	for _, f := range files {
		if data, err := os.ReadFile(f); err == nil {
			if mig := migrateColonTokens(string(data)); mig != string(data) {
				_ = os.WriteFile(f, []byte(mig), 0o644)
			}
		}
	}
	_, _ = MigrateTokens(path, true)
	_, _ = SplitToFragments(path, false)
	return removed, nil
}
