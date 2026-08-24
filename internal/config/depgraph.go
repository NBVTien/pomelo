package config

import "fmt"

type DepGraph struct {
	nodes map[string][]string
}

func BuildDepGraph(dir *Dir) *DepGraph {
	g := &DepGraph{nodes: make(map[string][]string)}
	for _, svcName := range dir.ServiceOrder {
		svc := dir.Services[svcName]
		if svc == nil {
			continue
		}
		g.nodes[svcName] = svc.DependsOn
	}
	return g
}

func (g *DepGraph) StartOrder(requested []string) ([]string, error) {
	if !g.hasDeps() {
		return requested, nil
	}
	return g.toposort(requested)
}

func (g *DepGraph) StopOrder(requested []string) ([]string, error) {
	order, err := g.StartOrder(requested)
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}
	return order, nil
}

func (g *DepGraph) hasDeps() bool {
	for _, deps := range g.nodes {
		if len(deps) > 0 {
			return true
		}
	}
	return false
}

func (g *DepGraph) toposort(requested []string) ([]string, error) {
	all := g.collectTransitive(requested)
	inDeg := make(map[string]int)
	adj := make(map[string][]string)

	for _, node := range all {
		if _, ok := inDeg[node]; !ok {
			inDeg[node] = 0
		}
		for _, dep := range g.nodes[node] {
			if contains(all, dep) {
				adj[dep] = append(adj[dep], node)
				inDeg[node]++
			}
		}
	}

	var queue []string
	for _, node := range all {
		if inDeg[node] == 0 {
			queue = append(queue, node)
		}
	}

	var result []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)
		for _, dependent := range adj[node] {
			inDeg[dependent]--
			if inDeg[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(result) != len(all) {
		return nil, fmt.Errorf("dependency cycle detected among services")
	}
	return result, nil
}

func (g *DepGraph) collectTransitive(requested []string) []string {
	seen := make(map[string]bool)
	var result []string
	var visit func(string)
	visit = func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		for _, dep := range g.nodes[name] {
			visit(dep)
		}
		result = append(result, name)
	}
	for _, name := range requested {
		visit(name)
	}
	return result
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
