package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func LintRedundantDefaults(path string) (map[string][]string, error) {
	data, _, err := MergedYAML(path)
	if err != nil {
		return nil, err
	}
	var raw Config
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return redundantSharedDefaults(&raw), nil
}

func redundantSharedDefaults(raw *Config) map[string][]string {
	tmpls := wellKnownServices()
	out := map[string][]string{}
	for name, def := range raw.SharedServices {
		if def == nil {
			continue
		}
		kind := def.Type
		if kind == "" {
			kind = name
		}
		t, ok := tmpls[kind]
		if !ok {
			continue
		}
		var red []string
		if def.Image != "" && def.Image == t.Image {
			red = append(red, "image")
		}
		if def.Command != "" && def.Command == t.Command {
			red = append(red, "command")
		}
		if len(def.Ports) > 0 && strSliceEqual(def.Ports, t.Ports) {
			red = append(red, "ports")
		}
		if len(def.Volumes) > 0 && strSliceEqual(def.Volumes, t.Volumes) {
			red = append(red, "volumes")
		}
		if def.DBUser != "" && def.DBUser == t.DBUser {
			red = append(red, "db_user")
		}
		if def.DBPassword != "" && def.DBPassword == t.DBPassword {
			red = append(red, "db_password")
		}
		for k, v := range def.Environment {
			if tv, ok := t.Environment[k]; ok && tv == v {
				red = append(red, "environment."+k)
			}
		}
		if len(red) > 0 {
			out[name] = red
		}
	}
	return out
}

type DupEnv struct {
	Key   string   `json:"key"`
	Value string   `json:"value"`
	Repos []string `json:"repos"`
}

func AnalyzeDuplicateEnv(path string) ([]DupEnv, error) {
	data, _, err := MergedYAML(path)
	if err != nil {
		return nil, err
	}
	var raw Config
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	type kv struct{ k, v string }
	repos := map[kv][]string{}
	for name, dir := range raw.Repos {
		if dir == nil {
			continue
		}
		base, _ := parseEnv(&dir.RawEnv)
		for k, v := range base {
			repos[kv{k, v}] = append(repos[kv{k, v}], name)
		}
	}
	var out []DupEnv
	for key, rs := range repos {
		if len(rs) >= 2 {
			sort.Strings(rs)
			out = append(out, DupEnv{Key: key.k, Value: key.v, Repos: rs})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Repos) != len(out[j].Repos) {
			return len(out[i].Repos) > len(out[j].Repos)
		}
		return out[i].Key < out[j].Key
	})
	return out, nil
}

type InlineCmdEnv struct {
	Service string   `json:"service"`
	Keys    []string `json:"keys"`
}

func LintInlineCmdEnv(cfg *Config) []InlineCmdEnv {
	if cfg == nil {
		return nil
	}
	var out []InlineCmdEnv
	for _, name := range cfg.RepoOrder {
		dir := cfg.Repos[name]
		if dir == nil {
			continue
		}
		alias := dir.Alias
		if alias == "" {
			alias = name
		}
		for _, svcName := range dir.ServiceOrder {
			svc := dir.Services[svcName]
			if svc == nil {
				continue
			}
			if keys := leadingEnvKeys(svc.ActiveCmd("")); len(keys) > 0 {
				out = append(out, InlineCmdEnv{Service: alias + "/" + svcName, Keys: keys})
			}
		}
	}
	return out
}

func leadingEnvKeys(cmd string) []string {
	var keys []string
	for _, tok := range strings.Fields(cmd) {
		eq := strings.IndexByte(tok, '=')
		if eq <= 0 {
			break
		}
		key := tok[:eq]
		if !isEnvKeyName(key) {
			break
		}
		keys = append(keys, key)
	}
	return keys
}

func isEnvKeyName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == '_' || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9' && i > 0) {
			continue
		}
		return false
	}
	return true
}

type LegacyToken struct {
	Old   string
	New   string
	Count int
}

func LintLegacyTokens(path string) ([]LegacyToken, error) {
	data, _, err := MergedYAML(path)
	if err != nil {
		return nil, err
	}
	text := string(data)
	out := []LegacyToken{}
	for _, lt := range []LegacyToken{
		{Old: "{{branch_safe}}", New: "{{branch|safe}}"},
		{Old: "{{branch_hash}}", New: "{{branch|hash}}"},
		{Old: "{{branch_host}}", New: "{{branch|host}}"},
	} {
		if n := strings.Count(text, lt.Old); n > 0 {
			lt.Count = n
			out = append(out, lt)
		}
	}
	return out, nil
}

func LintDeprecatedKeys(path string) ([]string, error) {
	data, _, err := MergedYAML(path)
	if err != nil {
		return nil, err
	}
	var raw Config
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := []string{}
	if len(raw.Combinations) > 0 {
		out = append(out, "`combinations:` is a deprecated alias for `workspaces:` — rename it (identical meaning)")
	}
	return out, nil
}

func MigrateTokens(path string, write bool) (map[string]int, error) {
	repl := strings.NewReplacer(
		"{{branch_safe}}", "{{branch|safe}}",
		"{{branch_hash}}", "{{branch|hash}}",
		"{{branch_host}}", "{{branch|host}}",
	)
	files := append([]string{path}, FragmentPaths(filepath.Dir(path))...)
	changes := map[string]int{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		before := string(data)
		n := strings.Count(before, "{{branch_safe}}") + strings.Count(before, "{{branch_hash}}") + strings.Count(before, "{{branch_host}}")
		if n == 0 {
			continue
		}
		changes[f] = n
		if write {
			if err := os.WriteFile(f, []byte(repl.Replace(before)), 0o644); err != nil {
				return changes, err
			}
		}
	}
	return changes, nil
}

func strSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
