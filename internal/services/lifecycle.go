package services

import "github.com/pomelohq/pomelo/internal/config"

type LifecycleStep struct {
	Cmd string `json:"cmd"`
}

type LifecycleStage struct {
	Name    string          `json:"name"`
	Trigger string          `json:"trigger"`
	Steps   []LifecycleStep `json:"steps"`
}

type LifecycleShortcut struct {
	Desc string `json:"desc"`
	Cmd  string `json:"cmd"`
	Key  string `json:"key,omitempty"`
}

type LifecycleView struct {
	Repo      string              `json:"repo"`
	Alias     string              `json:"alias"`
	Stages    []LifecycleStage    `json:"stages"`
	Shortcuts []LifecycleShortcut `json:"shortcuts"`
}

func LifecycleOf(cfg *config.Config, repo string) (*LifecycleView, bool) {
	name, dir := findRepoByNameOrAlias(cfg, repo)
	if dir == nil {
		return nil, false
	}
	alias := dir.Alias
	if alias == "" {
		alias = name
	}
	lv := &LifecycleView{Repo: name, Alias: alias}

	stage := func(n, trigger string, cmds []string) {
		if len(cmds) == 0 {
			return
		}
		st := LifecycleStage{Name: n, Trigger: trigger}
		for _, c := range cmds {
			st.Steps = append(st.Steps, LifecycleStep{Cmd: c})
		}
		lv.Stages = append(lv.Stages, st)
	}
	if len(dir.Copy) > 0 {
		stage("copy", "on create — files copied into the worktree", dir.Copy)
	}
	stage("setup", "on create — after worktree + env ready", dir.EffectiveSetup())
	stage("seed", "on create — after setup (skip with --no-seed)", dir.Seed)
	stage("migrate", "on refresh_main — when main gets new commits", dir.EffectiveMigrate())
	if dir.PreStart != "" {
		stage("pre_start", "before each service start", []string{dir.PreStart})
	}
	stage("pre_delete", "on workspace delete", dir.PreDelete)

	for _, sc := range dir.EffectiveShortcuts() {
		lv.Shortcuts = append(lv.Shortcuts, LifecycleShortcut{Desc: sc.Desc, Cmd: sc.Cmd, Key: sc.Key})
	}
	return lv, true
}
