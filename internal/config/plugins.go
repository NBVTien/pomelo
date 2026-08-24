package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func DecodePlugin[T any](blocks map[string]yaml.Node, name string) (*T, error) {
	node, ok := blocks[name]
	if !ok || node.Kind == 0 {
		return nil, nil
	}
	var out T
	if err := node.Decode(&out); err != nil {
		return nil, fmt.Errorf("decode plugin %q config: %w", name, err)
	}
	return &out, nil
}
