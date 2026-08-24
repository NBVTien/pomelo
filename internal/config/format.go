package config

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"sort"

	"gopkg.in/yaml.v3"
)

func FormatFile(path string, sortEnv bool) (out []byte, changed bool, err error) {
	orig, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(orig, &doc); err != nil {
		return nil, false, fmt.Errorf("parse %s: %w", path, err)
	}
	if sortEnv {
		sortEnvBlocks(&doc)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, false, err
	}
	_ = enc.Close()
	formatted := buf.Bytes()

	if !sameData(orig, formatted) {
		return nil, false, fmt.Errorf("format would change data in %s — refusing (please report)", path)
	}
	return formatted, !bytes.Equal(orig, formatted), nil
}

func sameData(a, b []byte) bool {
	var da, db any
	if yaml.Unmarshal(a, &da) != nil || yaml.Unmarshal(b, &db) != nil {
		return false
	}
	return reflect.DeepEqual(da, db)
}

func sortEnvBlocks(n *yaml.Node) {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			sortEnvBlocks(c)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, val := n.Content[i], n.Content[i+1]
			if key.Value == "env" && val.Kind == yaml.MappingNode {
				sortEnvMapping(val)
			}
			sortEnvBlocks(val)
		}
	}
}

func sortEnvMapping(m *yaml.Node) {
	sortMappingByKey(m)
	for i := 1; i < len(m.Content); i += 2 {
		if m.Content[i].Kind == yaml.MappingNode {
			sortMappingByKey(m.Content[i])
		}
	}
}

func sortMappingByKey(m *yaml.Node) {
	type kv struct{ k, v *yaml.Node }
	pairs := make([]kv, 0, len(m.Content)/2)
	for i := 0; i+1 < len(m.Content); i += 2 {
		pairs = append(pairs, kv{m.Content[i], m.Content[i+1]})
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].k.Value < pairs[j].k.Value })
	m.Content = m.Content[:0]
	for _, p := range pairs {
		m.Content = append(m.Content, p.k, p.v)
	}
}
