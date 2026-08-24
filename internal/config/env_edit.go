package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func SetRepoEnv(path, repo string, kv map[string]string) error {
	return editRepoEnv(path, repo, func(base *yaml.Node) {
		for k, v := range kv {
			setMapChild(base, k, scalar(v))
		}
	})
}

func UnsetRepoEnv(path, repo string, keys []string) error {
	return editRepoEnv(path, repo, func(base *yaml.Node) {
		for _, k := range keys {
			delMapChild(base, k)
		}
	})
}

func editRepoEnv(path, repo string, mutate func(base *yaml.Node)) error {
	files := append([]string{path}, FragmentPaths(dirOf(path))...)
	f := fileWithRepo(files, repo)
	if f == "" {
		return fmt.Errorf("repo %q not found in any config file", repo)
	}
	return editFile(f, func(root *yaml.Node) {
		repos := mapChild(root, "repos")
		rn := mapChild(repos, repo)
		if rn == nil {
			return
		}
		env := mapChild(rn, "env")
		if env == nil {
			env = &yaml.Node{Kind: yaml.MappingNode}
			setMapChild(rn, "env", env)
		}
		mutate(repoEnvBase(env))
	})
}

func SetEnvOverride(path, profile, key, value string) error {
	files := append([]string{path}, FragmentPaths(dirOf(path))...)
	f := fileWithTopKey(files, "environments")
	if f == "" {
		f = path
	}
	return editFile(f, func(root *yaml.Node) {
		envs := mapChild(root, "environments")
		if envs == nil {
			envs = &yaml.Node{Kind: yaml.MappingNode}
			setMapChild(root, "environments", envs)
		}
		p := mapChild(envs, profile)
		if p == nil {
			p = &yaml.Node{Kind: yaml.MappingNode}
			setMapChild(envs, profile, p)
		}
		setMapChild(p, key, scalar(value))
	})
}

func UnsetEnvOverride(path, profile, key string) error {
	files := append([]string{path}, FragmentPaths(dirOf(path))...)
	f := fileWithTopKey(files, "environments")
	if f == "" {
		return nil
	}
	return editFile(f, func(root *yaml.Node) {
		if p := mapChild(mapChild(root, "environments"), profile); p != nil {
			delMapChild(p, key)
		}
	})
}

func repoEnvBase(env *yaml.Node) *yaml.Node {
	fileKeyed := false
	for i := 1; i < len(env.Content); i += 2 {
		if env.Content[i].Kind == yaml.MappingNode {
			fileKeyed = true
			break
		}
	}
	if !fileKeyed {
		return env
	}
	if base := mapChild(env, "*"); base != nil {
		return base
	}
	base := &yaml.Node{Kind: yaml.MappingNode}
	env.Content = append([]*yaml.Node{scalar("*"), base}, env.Content...)
	return base
}
