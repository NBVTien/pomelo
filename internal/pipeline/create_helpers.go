package pipeline

import (
	"path/filepath"

	"github.com/pomelohq/pomelo/internal/config"
	"github.com/pomelohq/pomelo/internal/services"
)

func findDirPath(ctx *CreateContext, dirName string) string {
	for _, dp := range ctx.DirPaths {
		if dp.Name == dirName {
			return dp.Path
		}
	}
	return ""
}

func resolveTargetBranch(ctx *CreateContext, dirName string) string {
	if ctx.SelectedDirs != nil {
		for _, sd := range ctx.SelectedDirs {
			if sd.Name == dirName && sd.Branch != "" && sd.Branch != ctx.Branch {
				return sd.Branch
			}
		}
	}
	if ctx.GitBranch != "" && ctx.GitBranch != ctx.Branch {
		return ctx.GitBranch
	}
	return ctx.Branch
}

func createDatabases(ctx *CreateContext, branchSafe, branch string) {
	var dbNames []string
	pgSvc := findPGService(ctx.Config)
	host := ctx.Config.SharedHost("postgres")
	port := uint16(services.SharedPort("postgres"))
	if port == 0 {
		port = 5432
	}
	user := "postgres"
	pw := "postgres"
	if pgSvc != nil {
		if pgSvc.Host != "" {
			host = pgSvc.Host
		}
		if pgSvc.DBUser != "" {
			user = pgSvc.DBUser
		}
		if pgSvc.DBPassword != "" {
			pw = pgSvc.DBPassword
		}
	}

	defBranch := ctx.Config.GlobalDefaultBranch()
	type clonePair struct{ ws, main string }
	var clones []clonePair

	collect := func(tpl string, prefix bool) (wsName, mainName string) {
		wsName = services.ResolveBranchTokens(tpl, branch)
		mainName = services.ResolveBranchTokens(tpl, defBranch)
		if prefix {
			wsName = ctx.Session + "_" + wsName
			mainName = ctx.Session + "_" + mainName
		}
		return
	}

	for _, dirName := range ctx.UniqueDirs {
		dir := ctx.Config.Repos[dirName]
		if dir == nil || !dir.HasWorktreeConfig() {
			continue
		}
		add := func(tpl string, prefix bool) {
			ws, main := collect(tpl, prefix)
			if dir.SeedFromMain && main != ws {
				clones = append(clones, clonePair{ws, main})
			} else {
				dbNames = append(dbNames, ws)
			}
		}
		for _, sref := range dir.SharedSvcRefs {
			if sref.DBName != "" {
				add(sref.DBName, false)
			}
		}
		for _, dbTpl := range dir.Databases {
			add(dbTpl, true)
		}
	}

	if len(dbNames) > 0 {
		services.CreateSharedDBsBatch(host, port, dbNames, user, pw)
	}
	for _, c := range clones {
		if services.DatabaseExists(host, port, user, pw, c.main) {
			if err := services.CloneDatabase(host, port, user, pw, c.main, c.ws); err != nil {
				ctx.progress("db clone " + c.ws + " ← main failed: " + err.Error())
				_ = services.CreateDatabase(host, port, user, pw, c.ws)
			} else {
				ctx.progress("db " + c.ws + " ← main")
			}
		} else {
			_ = services.CreateDatabase(host, port, user, pw, c.ws)
		}
	}
}

func findPGService(cfg *config.Config) *config.SharedServiceDef {
	for _, svc := range cfg.SharedServices {
		if svc.DBUser != "" {
			return svc
		}
	}
	return nil
}

func applyAllEnvFiles(d *config.Dir, dirPath string, cfg *config.Config, branch, wsKey, envName string) {
	branchSafe := services.BranchSafe(branch)
	dbNames := make(map[string]string, len(d.Databases))
	for key, tpl := range d.Databases {
		dbNames[key] = cfg.Session + "_" + services.ResolveBranchTokens(tpl, branch)
	}

	baseEnv := make(map[string]string)
	for k, v := range d.Env {
		baseEnv[k] = v
	}

	var skipDirs []string
	for _, svcName := range d.ServiceOrder {
		if svc := d.Services[svcName]; svc != nil && svc.Dir != "" {
			skipDirs = append(skipDirs, svc.Dir)
		}
	}

	for _, entry := range d.EnvFileEntries() {
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
		resolved := services.ResolveEnvTemplates(envSrc, cfg, branchSafe, branch, wsKey, envName, dbNames)
		services.ApplyEnvOverrides(dirPath, resolved, entry.File, skipDirs...)
	}

	envFile := ".env.local"
	if entries := d.EnvFileEntries(); len(entries) > 0 {
		envFile = entries[0].File
	}
	for _, svcName := range d.ServiceOrder {
		svc := d.Services[svcName]
		if svc == nil || svc.Dir == "" {
			continue
		}
		svcEnv := make(map[string]string)
		for k, v := range d.Env {
			svcEnv[k] = v
		}
		for k, v := range svc.Env {
			svcEnv[k] = v
		}
		if len(svcEnv) == 0 {
			continue
		}
		resolved := services.ResolveEnvTemplates(svcEnv, cfg, branchSafe, branch, wsKey, envName, dbNames)
		svcDir := filepath.Join(dirPath, svc.Dir)
		services.ApplyEnvOverridesToDir(svcDir, resolved, envFile)
	}
}
