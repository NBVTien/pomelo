package codeagent

import "github.com/pomelohq/pomelo/internal/config"

func init() {
	config.RegisterPostLoadHook(Inject)
}

func Inject(cfg *config.Config) {
	if cfg == nil {
		return
	}
	if cfg.CodeAgents != nil && cfg.CodeAgents.Disabled {
		return
	}
	if cfg.WsServices == nil {
		cfg.WsServices = make(map[string]*config.Service)
	}

	for _, agent := range builtinFiltered(cfg) {
		if _, exists := cfg.WsServices[agent.Name]; exists {
			continue
		}
		portFalse := false
		cfg.WsServices[agent.Name] = &config.Service{
			Cmd:  agent.Cmd,
			Port: &portFalse,
		}
		cfg.WsServiceOrder = append(cfg.WsServiceOrder, agent.Name)
	}
}

func builtinFiltered(cfg *config.Config) []*CodeAgent {
	all := Builtin()
	if cfg.CodeAgents == nil || len(cfg.CodeAgents.Only) == 0 {
		return all
	}
	allowed := make(map[string]bool, len(cfg.CodeAgents.Only))
	for _, n := range cfg.CodeAgents.Only {
		allowed[n] = true
	}
	filtered := make([]*CodeAgent, 0, len(all))
	for _, a := range all {
		if allowed[a.Name] {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

func LookupAgent(serviceName string) *CodeAgent {
	for _, a := range Builtin() {
		if a.Name == serviceName {
			return a
		}
	}
	return nil
}
