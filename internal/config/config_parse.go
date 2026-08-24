package config

import "gopkg.in/yaml.v3"

func parsePresetField(node *yaml.Node) []string {
	if node == nil || node.Kind == 0 {
		return nil
	}
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Value != "" {
			return []string{node.Value}
		}
	case yaml.SequenceNode:
		var result []string
		for _, item := range node.Content {
			if item.Kind == yaml.ScalarNode && item.Value != "" {
				result = append(result, item.Value)
			}
		}
		return result
	}
	return nil
}

func extractRepoOrder(cfg *Config, root *yaml.Node) {
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i+1 < len(doc.Content); i += 2 {
		key := doc.Content[i].Value
		val := doc.Content[i+1]
		if key == "shared_services" && val.Kind == yaml.MappingNode {
			for j := 0; j+1 < len(val.Content); j += 2 {
				cfg.SharedOrder = append(cfg.SharedOrder, val.Content[j].Value)
			}
		}
		if (key == "repos" || key == "dirs") && val.Kind == yaml.MappingNode {
			for j := 0; j+1 < len(val.Content); j += 2 {
				repoName := val.Content[j].Value
				cfg.RepoOrder = append(cfg.RepoOrder, repoName)
				if dir, ok := cfg.Repos[repoName]; ok {
					svcNode := val.Content[j+1]
					if svcNode.Kind == yaml.MappingNode {
						for k := 0; k+1 < len(svcNode.Content); k += 2 {
							if svcNode.Content[k].Value == "services" && svcNode.Content[k+1].Kind == yaml.MappingNode {
								svcs := svcNode.Content[k+1]
								for s := 0; s+1 < len(svcs.Content); s += 2 {
									dir.ServiceOrder = append(dir.ServiceOrder, svcs.Content[s].Value)
								}
							}
						}
					}
				}
			}
		}
	}
}

func parseEnv(node *yaml.Node) (base map[string]string, files []EnvFileEntry) {
	if node == nil || node.Kind != yaml.MappingNode || len(node.Content) == 0 {
		return nil, nil
	}

	fileKeyed := false
	for i := 1; i < len(node.Content); i += 2 {
		if node.Content[i].Kind == yaml.MappingNode {
			fileKeyed = true
			break
		}
	}

	flatMap := func(n *yaml.Node) map[string]string {
		m := make(map[string]string)
		for j := 0; j+1 < len(n.Content); j += 2 {
			m[n.Content[j].Value] = n.Content[j+1].Value
		}
		return m
	}

	if !fileKeyed {
		base = flatMap(node)
		return base, []EnvFileEntry{{File: ".env.local"}}
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		if val.Kind != yaml.MappingNode {
			continue
		}
		if key == "*" {
			base = flatMap(val)
			continue
		}
		files = append(files, EnvFileEntry{File: key, Env: flatMap(val)})
	}
	return base, files
}

func parseSharedRefs(node *yaml.Node) []SharedServiceRef {
	if node == nil || node.Kind == 0 || node.Kind != yaml.SequenceNode {
		return nil
	}
	var result []SharedServiceRef
	for _, item := range node.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			result = append(result, SharedServiceRef{Name: item.Value})
		case yaml.MappingNode:
			for i := 0; i+1 < len(item.Content); i += 2 {
				name := item.Content[i].Value
				ref := SharedServiceRef{Name: name}
				val := item.Content[i+1]
				if val.Kind == yaml.MappingNode {
					for j := 0; j+1 < len(val.Content); j += 2 {
						if val.Content[j].Value == "db_name" {
							ref.DBName = val.Content[j+1].Value
						}
					}
				}
				result = append(result, ref)
			}
		}
	}
	return result
}
