package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const FragmentDir = "pom.d"

func loadMergedYAML(path string) ([]byte, error) {
	data, _, err := MergedYAML(path)
	return data, err
}

func MergedYAML(path string) (data []byte, split bool, err error) {
	root, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read %s: %w", path, err)
	}
	frags, err := fragmentFiles(filepath.Join(filepath.Dir(path), FragmentDir))
	if err != nil || len(frags) == 0 {
		return root, false, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(root, &doc); err != nil {
		return nil, false, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	dst := mappingOf(&doc)

	for _, f := range frags {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, false, fmt.Errorf("read fragment %s: %w", f, err)
		}
		var fdoc yaml.Node
		if err := yaml.Unmarshal(b, &fdoc); err != nil {
			return nil, false, fmt.Errorf("parse fragment %s: %w", f, err)
		}
		src := mappingOf(&fdoc)
		if src == nil {
			continue
		}
		mergeMapping(dst, src)
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, false, fmt.Errorf("marshal merged config: %w", err)
	}
	return out, true, nil
}

func fragmentFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			if p != dir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if ext := strings.ToLower(filepath.Ext(d.Name())); ext == ".yml" || ext == ".yaml" {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func FragmentPaths(configDir string) []string {
	files, _ := fragmentFiles(filepath.Join(configDir, FragmentDir))
	return files
}

func mappingOf(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			doc.Content = []*yaml.Node{m}
			return m
		}
		if doc.Content[0].Kind == yaml.MappingNode {
			return doc.Content[0]
		}
	}
	return nil
}

func mergeMapping(dst, src *yaml.Node) {
	for i := 0; i+1 < len(src.Content); i += 2 {
		key, val := src.Content[i], src.Content[i+1]
		if j := findKey(dst, key.Value); j >= 0 {
			cur := dst.Content[j+1]
			if cur.Kind == yaml.MappingNode && val.Kind == yaml.MappingNode {
				mergeMapping(cur, val)
			} else {
				dst.Content[j+1] = val
			}
			continue
		}
		dst.Content = append(dst.Content, key, val)
	}
}

func findKey(m *yaml.Node, name string) int {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == name {
			return i
		}
	}
	return -1
}

type SplitResult struct {
	RootFile   string
	Fragments  []string
	BackupFile string
}

type fragPlan struct {
	file string
	data []byte
}

func SplitToFragments(path string, dryRun bool) (*SplitResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	root := mappingOf(&doc)
	if root == nil {
		return nil, fmt.Errorf("%s is not a YAML mapping", path)
	}

	dir := filepath.Dir(path)
	fragDir := filepath.Join(dir, FragmentDir)
	res := &SplitResult{BackupFile: path + ".bak", RootFile: path}

	var plan []fragPlan
	moved := false

	if ri := findKey(root, "repos"); ri >= 0 {
		repos := root.Content[ri+1]
		if repos.Kind == yaml.MappingNode && len(repos.Content) > 0 {
			for i := 0; i+1 < len(repos.Content); i += 2 {
				name := repos.Content[i].Value
				inner := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map",
					Content: []*yaml.Node{repos.Content[i], repos.Content[i+1]}}
				out, err := yaml.Marshal(wrapDoc("repos", inner))
				if err != nil {
					return nil, fmt.Errorf("marshal repo %s: %w", name, err)
				}
				file := filepath.Join(fragDir, "repos", fmt.Sprintf("%02d-%s.yml", i/2+1, safeFragName(name)))
				plan = append(plan, fragPlan{file, out})
				res.Fragments = append(res.Fragments, file)
			}
			removeKey(root, ri)
			moved = true
		}
	}

	for _, sec := range []struct{ key, file string }{
		{"environments", "environments.yml"},
		{"presets", "presets.yml"},
		{"shared_services", "shared-services.yml"},
	} {
		ki := findKey(root, sec.key)
		if ki < 0 {
			continue
		}
		out, err := yaml.Marshal(wrapDoc(sec.key, root.Content[ki+1]))
		if err != nil {
			return nil, fmt.Errorf("marshal %s: %w", sec.key, err)
		}
		file := filepath.Join(fragDir, sec.file)
		plan = append(plan, fragPlan{file, out})
		res.Fragments = append(res.Fragments, file)
		removeKey(root, ki)
		moved = true
	}

	if !moved {
		return nil, fmt.Errorf("nothing to split in %s (no repos/environments/presets/shared_services)", path)
	}

	newRoot, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, fmt.Errorf("marshal root: %w", err)
	}
	if dryRun {
		return res, nil
	}

	if err := os.WriteFile(res.BackupFile, data, 0o644); err != nil {
		return nil, fmt.Errorf("backup: %w", err)
	}
	if entries, err := os.ReadDir(filepath.Join(fragDir, "repos")); err == nil {
		for _, e := range entries {
			if !e.IsDir() && splitRepoFragment.MatchString(e.Name()) {
				_ = os.Remove(filepath.Join(fragDir, "repos", e.Name()))
			}
		}
	}
	for _, f := range plan {
		if err := os.MkdirAll(filepath.Dir(f.file), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(f.file, f.data, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", f.file, err)
		}
	}
	if err := os.WriteFile(res.RootFile, newRoot, 0o644); err != nil {
		return nil, fmt.Errorf("write root %s: %w", res.RootFile, err)
	}
	return res, nil
}

func wrapDoc(key string, val *yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{
		{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, val,
		}},
	}}
}

func removeKey(m *yaml.Node, i int) {
	m.Content = append(m.Content[:i], m.Content[i+2:]...)
}

var splitRepoFragment = regexp.MustCompile(`^\d\d-.*\.ya?ml$`)

func safeFragName(repo string) string {
	return strings.NewReplacer("/", "-", " ", "-").Replace(repo)
}
