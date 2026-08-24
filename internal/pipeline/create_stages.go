package pipeline

import (
	"fmt"
	"github.com/pomelohq/pomelo/internal/provider/shell"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pomelohq/pomelo/internal/services"
)

func stageValidate(ctx *CreateContext) error {
	return nil
}

func stageProvision(ctx *CreateContext, state *CreateState) error {
	if len(ctx.Config.SharedServices) > 0 {
		allocateSharedSlots(ctx)
	}

	state.WsFolder = services.EnsureWorkspaceFolder(ctx.ConfigDir, ctx.Branch)
	if ctx.Environment != "" {
		svcEnvs := make(map[string]string)
		for _, dirName := range ctx.UniqueDirs {
			alias := dirName
			if dir, ok := ctx.Config.Repos[dirName]; ok && dir.Alias != "" {
				alias = dir.Alias
			}
			svcEnvs[alias] = ctx.Environment
		}
		services.SaveWorkspaceState(state.WsFolder, &services.WorkspaceState{
			ServiceEnvs: svcEnvs,
		})
	}
	return nil
}

func allocateSharedSlots(ctx *CreateContext) {
	wsKey := services.PortWsKey(ctx.Branch)
	allocated := make(map[string]bool)

	for _, dirName := range ctx.UniqueDirs {
		dir, ok := ctx.Config.Repos[dirName]
		if !ok || !dir.HasWorktreeConfig() {
			continue
		}
		for _, sref := range dir.SharedSvcRefs {
			if allocated[sref.Name] {
				continue
			}
			if svcDef, ok := ctx.Config.SharedServices[sref.Name]; ok && svcDef.Capacity != nil {
				basePort := uint16(services.SharedPort(sref.Name))
				services.AllocateSlot(sref.Name, wsKey, *svcDef.Capacity, basePort)
				allocated[sref.Name] = true
			}
		}
		for _, val := range dir.Env {
			for _, svcName := range services.SlotRefsIn(val) {
				if allocated[svcName] {
					continue
				}
				if svcDef, ok := ctx.Config.SharedServices[svcName]; ok && svcDef.Capacity != nil {
					basePort := uint16(services.SharedPort(svcName))
					services.AllocateSlot(svcName, wsKey, *svcDef.Capacity, basePort)
					allocated[svcName] = true
				}
			}
		}
	}
}

func stageInfra(ctx *CreateContext, state *CreateState) error {
	if len(ctx.Config.SharedServices) == 0 {
		return nil
	}

	var allServices []string
	for name := range ctx.Config.SharedServices {
		allServices = append(allServices, name)
	}
	services.GenerateSharedCompose(ctx.ConfigDir, ctx.Session, ctx.Config.SharedServices)
	ctx.progress("starting: " + strings.Join(allServices, ", "))
	services.StartSharedServices(ctx.ConfigDir, ctx.Session, allServices)

	ctx.progress("creating databases")
	createDatabases(ctx, state.BranchSafe, ctx.Branch)
	return nil
}

func stageSourceParallel(ctx *CreateContext, state *CreateState) error {
	var mu sync.Mutex
	var errs []error
	var wg sync.WaitGroup

	for _, db := range ctx.DirBranches {
		dirName, baseBranch := db.Name, db.Branch
		dirPath := findDirPath(ctx, dirName)
		if dirPath == "" {
			continue
		}

		targetBranch := resolveTargetBranch(ctx, dirName)

		dir := ctx.Config.Repos[dirName]
		var copyFiles []string
		if dir != nil && dir.HasWorktreeConfig() {
			copyFiles = dir.Copy
		}

		checkoutBranch := ""
		if targetBranch != ctx.Branch {
			checkoutBranch = targetBranch
			targetBranch = ctx.Branch
		}

		wg.Add(1)
		go func(dn, dp, tb, bb, cb string, cf []string) {
			defer wg.Done()
			ctx.progress("worktree: " + dn)
			wtPath, err := services.CreateWorktreeFromBase(dp, tb, bb, cf, state.WsFolder)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("failed to create worktree for %s: %w", dn, err))
				mu.Unlock()
				return
			}
			if cb != "" {
				if chErr := services.Checkout(wtPath, cb); chErr != nil {
					_ = exec.Command("git", "-C", wtPath, "fetch", "origin", cb).Run()
					_ = services.Checkout(wtPath, cb)
				}
			}
			mu.Lock()
			state.WtDirs = append(state.WtDirs, services.DirMapping{Name: dn, Path: wtPath})
			mu.Unlock()
		}(dirName, dirPath, targetBranch, baseBranch, checkoutBranch, copyFiles)
	}
	wg.Wait()

	if len(errs) > 0 {
		for _, dm := range state.WtDirs {
			_ = services.RemoveWorktree(dm.Path+"/..", dm.Path, ctx.Branch)
		}
		state.WtDirs = nil
		return errs[0]
	}
	return nil
}

func stageConfigureParallel(ctx *CreateContext, state *CreateState) error {
	if state.WsFolder == "" {
		state.WsFolder = filepath.Join(ctx.ConfigDir, "workspace--"+ctx.Branch)
	}
	if len(state.WtDirs) == 0 {
		for _, d := range ctx.UniqueDirs {
			wtPath := filepath.Join(state.WsFolder, d)
			if services.DirExists(wtPath) {
				state.WtDirs = append(state.WtDirs, services.DirMapping{Name: d, Path: wtPath})
			}
		}
	}

	var wg sync.WaitGroup
	for _, wd := range state.WtDirs {
		dirName, wtPath := wd.Name, wd.Path
		dir := ctx.Config.Repos[dirName]
		if dir == nil || !dir.HasWorktreeConfig() {
			continue
		}

		wg.Add(1)
		go func(wp string, dn string) {
			defer wg.Done()
			wsKey := services.PortWsKey(ctx.Branch)
			applyAllEnvFiles(ctx.Config.Repos[dn], wp, ctx.Config, ctx.Branch, wsKey, ctx.Environment)
		}(wtPath, dirName)
	}
	wg.Wait()

	services.EnsureGlobalGitignore()
	return nil
}

func stageSetupParallel(ctx *CreateContext, state *CreateState) error {
	defBranch := ctx.Config.GlobalDefaultBranch()
	nmShared := map[string]bool{}
	if ctx.Branch != defBranch {
		for _, wd := range state.WtDirs {
			if services.FileExists(filepath.Join(wd.Path, "pnpm-lock.yaml")) {
				continue
			}
			if services.EnsureNodeModulesFromStore(ctx.ConfigDir, wd.Name, wd.Path, defBranch) {
				nmShared[wd.Name] = true
				ctx.progress(wd.Name + ": node_modules ← store")
			}
		}
	}

	var wg sync.WaitGroup
	for _, wd := range state.WtDirs {
		dir := ctx.Config.Repos[wd.Name]
		if dir == nil || !dir.HasWorktreeConfig() {
			continue
		}
		steps := dir.EffectiveSetup()
		if len(steps) == 0 {
			continue
		}
		setupCmds := append([]string{}, steps...)
		combined := strings.Join(setupCmds, " && ")
		fresh := !nmShared[wd.Name]
		wg.Add(1)
		go func(name, wtPath, cmd string, fresh bool) {
			defer wg.Done()
			ctx.progress(name + ": setup…")
			login := shell.Login(cmd)
			c := exec.Command(login[0], login[1:]...)
			c.Dir = wtPath
			_ = c.Run()
			if fresh {
				services.SnapshotNodeModules(name, wtPath)
			}
			ctx.progress(name + ": setup done")
		}(wd.Name, wd.Path, combined, fresh)
	}
	wg.Wait()
	return nil
}

func stageSeedParallel(ctx *CreateContext, state *CreateState) error {
	runSeed := func(cmds []string, dir, label string) {
		if len(cmds) == 0 {
			return
		}
		login := shell.Login(strings.Join(cmds, " && "))
		c := exec.Command(login[0], login[1:]...)
		c.Dir = dir
		if o, err := c.CombinedOutput(); err != nil {
			fmt.Printf("  seed for %s failed (non-fatal): %v\n%s\n", label, err, o)
		}
	}

	if len(ctx.Config.Seed) > 0 {
		ctx.progress("workspace: seeding…")
	}
	runSeed(ctx.Config.Seed, services.WorkspaceRootDir(ctx.ConfigDir, ctx.Branch, false), "workspace")

	var wg sync.WaitGroup
	for _, wd := range state.WtDirs {
		dir := ctx.Config.Repos[wd.Name]
		if dir == nil || len(dir.Seed) == 0 || dir.SeedFromMain {
			continue
		}
		wg.Add(1)
		go func(name, wtPath string, cmds []string) {
			defer wg.Done()
			ctx.progress(name + ": seeding…")
			runSeed(cmds, wtPath, name)
			ctx.progress(name + ": seed done")
		}(wd.Name, wd.Path, dir.Seed)
	}
	wg.Wait()
	return nil
}
