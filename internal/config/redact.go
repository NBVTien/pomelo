package config

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var reSensitiveKey = regexp.MustCompile(`(?i)(token|secret|password|passwd|api_?key|_key$|_pk$|stripe|dsn|access_?key|private_?key|email|^site$)`)

func RedactYAML(data []byte) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	redactNode(&doc)
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func redactScalar(n *yaml.Node) {
	if n.Kind == yaml.ScalarNode {
		if strings.TrimSpace(n.Value) != "" {
			n.Value = "<redacted>"
			n.Tag = "!!str"
			n.Style = 0
		}
		return
	}
	for _, c := range n.Content {
		redactScalar(c)
	}
}

func redactEnvValues(envMap *yaml.Node) {
	for i := 0; i+1 < len(envMap.Content); i += 2 {
		profile := envMap.Content[i+1]
		if profile.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(profile.Content); j += 2 {
			redactScalar(profile.Content[j+1])
		}
	}
}

func redactNode(n *yaml.Node) {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			redactNode(c)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, val := n.Content[i], n.Content[i+1]
			lower := strings.ToLower(key.Value)
			if lower == "environments" && val.Kind == yaml.MappingNode {
				redactEnvValues(val)
				continue
			}
			if reSensitiveKey.MatchString(key.Value) && !strings.HasSuffix(lower, "_env") {
				redactScalar(val)
				continue
			}
			redactNode(val)
		}
	}
}
