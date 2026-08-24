package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func splitLeadingEnv(cmd string) (pairs [][2]string, bare string, ok bool) {
	i := 0
	for i < len(cmd) {
		for i < len(cmd) && cmd[i] == ' ' {
			i++
		}
		j := i
		for j < len(cmd) && cmd[j] != ' ' {
			j++
		}
		tok := cmd[i:j]
		eq := strings.IndexByte(tok, '=')
		if eq <= 0 || !isEnvKeyName(tok[:eq]) {
			break
		}
		if strings.ContainsAny(tok, "'\"") {
			return nil, "", false
		}
		pairs = append(pairs, [2]string{tok[:eq], tok[eq+1:]})
		i = j
	}
	bare = strings.TrimSpace(cmd[i:])
	return pairs, bare, len(pairs) > 0 && bare != ""
}

func ExtractCmdEnv(path string, write bool) (map[string][]string, error) {
	data, _, err := MergedYAML(path)
	if err != nil {
		return nil, err
	}
	var raw Config
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	plan := map[string][]string{}
	for name, dir := range raw.Repos {
		if dir == nil {
			continue
		}
		for svcName, svc := range dir.Services {
			if svc == nil || len(svc.Modes) > 0 {
				continue
			}
			if pairs, _, ok := splitLeadingEnv(svc.Cmd); ok {
				keys := make([]string, len(pairs))
				for i, p := range pairs {
					keys[i] = p[0]
				}
				plan[name+"/"+svcName] = keys
			}
		}
	}
	if !write || len(plan) == 0 {
		return plan, nil
	}

	files := append([]string{path}, FragmentPaths(dirOf(path))...)
	byRepo := map[string]bool{}
	for k := range plan {
		byRepo[strings.SplitN(k, "/", 2)[0]] = true
	}
	backups := map[string][]byte{}
	restore := func() {
		for f, b := range backups {
			_ = os.WriteFile(f, b, 0o644)
		}
	}
	for repo := range byRepo {
		f := fileWithRepo(files, repo)
		if f == "" {
			continue
		}
		if _, done := backups[f]; !done {
			b, err := os.ReadFile(f)
			if err != nil {
				restore()
				return nil, err
			}
			backups[f] = b
		}
		if err := editFile(f, func(root *yaml.Node) { extractRepoCmdEnv(root, repo) }); err != nil {
			restore()
			return nil, err
		}
	}

	cfg, err := Load(path)
	if err != nil {
		restore()
		return nil, err
	}
	for k, keys := range plan {
		repo, svcName, _ := strings.Cut(k, "/")
		dir := cfg.Repos[repo]
		if dir == nil || dir.Services[svcName] == nil {
			restore()
			return nil, fmt.Errorf("verify: %s vanished", k)
		}
		svc := dir.Services[svcName]
		if _, _, ok := splitLeadingEnv(svc.Cmd); ok {
			restore()
			return nil, fmt.Errorf("verify: %s cmd still has inline env — rolled back", k)
		}
		for _, key := range keys {
			if _, has := svc.Env[key]; !has {
				restore()
				return nil, fmt.Errorf("verify: %s env missing %s — rolled back", k, key)
			}
		}
	}
	return plan, nil
}

func extractRepoCmdEnv(root *yaml.Node, repo string) {
	repoNode := mapChild(mapChild(root, "repos"), repo)
	services := mapChild(repoNode, "services")
	if services == nil {
		return
	}
	for i := 0; i+1 < len(services.Content); i += 2 {
		svcVal := services.Content[i+1]
		var cmdNode *yaml.Node
		var svcMap *yaml.Node
		if svcVal.Kind == yaml.ScalarNode {
			cmdNode = svcVal
		} else if svcVal.Kind == yaml.MappingNode {
			svcMap = svcVal
			cmdNode = mapChild(svcVal, "cmd")
		}
		if cmdNode == nil {
			continue
		}
		pairs, bare, ok := splitLeadingEnv(cmdNode.Value)
		if !ok {
			continue
		}
		if svcMap == nil {
			svcMap = &yaml.Node{Kind: yaml.MappingNode}
			services.Content[i+1] = svcMap
		}
		env := mapChild(svcMap, "env")
		if env == nil {
			env = &yaml.Node{Kind: yaml.MappingNode}
			setMapChild(svcMap, "env", env)
		}
		for _, p := range pairs {
			setMapChild(env, p[0], scalar(p[1]))
		}
		setMapChild(svcMap, "cmd", scalar(bare))
	}
}
