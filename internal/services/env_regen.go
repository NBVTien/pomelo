package services

import (
	"os"
	"path/filepath"

	"github.com/pomelohq/pomelo/internal/config"
)

func RegenerateWorkspaceEnv(configDir string, cfg *config.Config, branch string) {
	wsKey := PortWsKey(branch)
	wsFolder := filepath.Join(configDir, "workspace--"+branch)

	if _, err := os.Stat(wsFolder); os.IsNotExist(err) {
		return
	}

	wsState := LoadWorkspaceState(wsFolder)

	allocateSlots(cfg, wsKey)
	acquireWorkspacePorts(cfg, wsKey)

	for _, dirName := range cfg.RepoOrder {
		dir := cfg.Repos[dirName]
		if dir == nil {
			continue
		}
		wtPath := filepath.Join(wsFolder, dirName)
		if _, err := os.Stat(wtPath); os.IsNotExist(err) {
			continue
		}

		if !dir.HasWorktreeConfig() {
			continue
		}

		alias := dir.Alias
		if alias == "" {
			alias = dirName
		}
		envName := ""
		if wsState.ServiceEnvs != nil {
			envName = wsState.ServiceEnvs[alias]
		}

		branchSafe := BranchSafe(branch)

		dbNames := make(map[string]string, len(dir.Databases))
		for key, tpl := range dir.Databases {
			dbNames[key] = cfg.Session + "_" + ResolveBranchTokens(tpl, branch)
		}

		baseEnv := make(map[string]string)
		for k, v := range dir.Env {
			baseEnv[k] = v
		}

		var skipDirs []string
		for _, svcName := range dir.ServiceOrder {
			if svc := dir.Services[svcName]; svc != nil && svc.Dir != "" {
				skipDirs = append(skipDirs, svc.Dir)
			}
		}

		for _, entry := range dir.EnvFileEntries() {
			envSrc := baseEnv
			if len(entry.Env) > 0 {
				envSrc = make(map[string]string)
				for k, v := range baseEnv {
					envSrc[k] = v
				}
				for k, v := range entry.Env {
					envSrc[k] = v
				}
			}
			resolved := ResolveEnvTemplates(envSrc, cfg, branchSafe, branch, wsKey, envName, dbNames)
			ApplyEnvOverrides(wtPath, resolved, entry.File, skipDirs...)
		}

		envFile := ".env.local"
		if entries := dir.EnvFileEntries(); len(entries) > 0 {
			envFile = entries[0].File
		}
		for _, svcName := range dir.ServiceOrder {
			svc := dir.Services[svcName]
			if svc == nil {
				continue
			}
			if svc.Dir == "" && len(svc.Env) == 0 {
				continue
			}
			svcEnv := make(map[string]string)
			for k, v := range dir.Env {
				svcEnv[k] = v
			}
			if svc.Dir == "" {
				for _, entry := range dir.EnvFileEntries() {
					if entry.File == envFile {
						for k, v := range entry.Env {
							svcEnv[k] = v
						}
					}
				}
			}
			for k, v := range svc.Env {
				svcEnv[k] = v
			}
			if len(svcEnv) == 0 {
				continue
			}
			svcEnvName := envName
			if wsState.ServiceEnvs != nil {
				if e, ok := wsState.ServiceEnvs[alias+"/"+svcName]; ok {
					svcEnvName = e
				} else if e, ok := wsState.ServiceEnvs[alias]; ok {
					svcEnvName = e
				}
			}
			resolved := ResolveEnvTemplates(svcEnv, cfg, branchSafe, branch, wsKey, svcEnvName, dbNames)
			svcDir := wtPath
			if svc.Dir != "" {
				svcDir = filepath.Join(wtPath, svc.Dir)
			}
			ApplyEnvOverridesToDir(svcDir, resolved, envFile)
		}
	}
}

func ResolveServiceEnv(configDir string, cfg *config.Config, branch, dirName, svcName string) []string {
	dir := cfg.Repos[dirName]
	if dir == nil {
		return nil
	}
	svc := dir.Services[svcName]
	if svc == nil {
		return nil
	}
	alias := dir.Alias
	if alias == "" {
		alias = dirName
	}
	wsKey := PortWsKey(branch)
	wsState := LoadWorkspaceState(filepath.Join(configDir, "workspace--"+branch))
	envName := ""
	if wsState.ServiceEnvs != nil {
		if e, ok := wsState.ServiceEnvs[alias+"/"+svcName]; ok {
			envName = e
		} else if e, ok := wsState.ServiceEnvs[alias]; ok {
			envName = e
		}
	}
	dbNames := make(map[string]string, len(dir.Databases))
	for key, tpl := range dir.Databases {
		dbNames[key] = cfg.Session + "_" + ResolveBranchTokens(tpl, branch)
	}
	merged := make(map[string]string, len(dir.Env)+len(svc.Env))
	for k, v := range dir.Env {
		merged[k] = v
	}
	for k, v := range svc.Env {
		merged[k] = v
	}
	resolved := ResolveEnvTemplates(merged, cfg, BranchSafe(branch), branch, wsKey, envName, dbNames)
	out := make([]string, 0, len(resolved))
	for _, e := range resolved {
		out = append(out, e.Key+"="+e.Value)
	}
	return out
}

func allocateSlots(cfg *config.Config, wsKey string) {
	allocated := make(map[string]bool)

	var allEnvValues []string
	for _, dir := range cfg.Repos {
		if dir == nil {
			continue
		}
		for _, sref := range dir.SharedSvcRefs {
			if allocated[sref.Name] {
				continue
			}
			if svcDef, ok := cfg.SharedServices[sref.Name]; ok && svcDef.Capacity != nil {
				basePort := uint16(SharedPort(sref.Name))
				AllocateSlot(sref.Name, wsKey, *svcDef.Capacity, basePort)
				allocated[sref.Name] = true
			}
		}
		for _, v := range dir.Env {
			allEnvValues = append(allEnvValues, v)
		}
	}

	for _, val := range allEnvValues {
		for _, svcName := range SlotRefsIn(val) {
			if allocated[svcName] {
				continue
			}
			if svcDef, ok := cfg.SharedServices[svcName]; ok && svcDef.Capacity != nil {
				AllocateSlot(svcName, wsKey, *svcDef.Capacity, uint16(SharedPort(svcName)))
				allocated[svcName] = true
			}
		}
	}
}
