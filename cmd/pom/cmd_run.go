package main

import (
	"fmt"
	"github.com/pomelohq/pomelo/internal/provider/shell"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/services"
)

var runCmd = &cobra.Command{
	Use:   "run <command> [repo]",
	Short: "Run a shortcut command in a repo directory",
	Long: `Run a command in a repo's worktree directory. Sources the primary env_output file before execution.

Examples:
  pom run "bundle exec rake db:migrate" api
  pom run "RACK_ENV=test bundle exec rspec spec/models" api
  pom run "npm test" client
  pom run "bundle install"                  # run in first repo`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		shellCmd := args[0]
		configDir := filepath.Dir(configPath)
		branch, _ := cmd.Flags().GetString("branch")
		if branch == "" {
			branch = appConfig.GlobalDefaultBranch()
		}

		var dirName string
		var workDir string
		if len(args) >= 2 {
			target := args[1]
			for dn, dir := range appConfig.Repos {
				if dn == target || dir.Alias == target {
					dirName = dn
					workDir = filepath.Join(configDir, "workspace--"+branch, dn)
					break
				}
			}
			if workDir == "" {
				return fmt.Errorf("repo '%s' not found", target)
			}
		} else {
			if len(appConfig.RepoOrder) > 0 {
				dirName = appConfig.RepoOrder[0]
				workDir = filepath.Join(configDir, "workspace--"+branch, dirName)
			}
		}

		if _, err := os.Stat(workDir); os.IsNotExist(err) {
			return fmt.Errorf("directory not found: %s", workDir)
		}

		if dir := appConfig.Repos[dirName]; dir != nil && !strings.ContainsAny(shellCmd, " \t") {
			for _, sc := range dir.EffectiveShortcuts() {
				if strings.EqualFold(sc.Key, shellCmd) {
					shellCmd = sc.Cmd
					break
				}
			}
		}

		services.RegenerateWorkspaceEnv(configDir, appConfig, branch)

		preStart := ""
		if dir := appConfig.Repos[dirName]; dir != nil && dir.PreStart != "" {
			preStart = dir.PreStart + " && "
		}

		fullCmd := fmt.Sprintf("cd '%s' && %s%s", workDir, preStart, shellCmd)
		sh := shell.Command(fullCmd)
		c := exec.Command(sh[0], sh[1:]...)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

var runListCmd = &cobra.Command{
	Use:   "shortcuts",
	Short: "List available shortcuts",
	Run: func(cmd *cobra.Command, args []string) {
		for _, dirName := range appConfig.RepoOrder {
			dir := appConfig.Repos[dirName]
			alias := dirName
			if dir.Alias != "" {
				alias = dir.Alias
			}
			scs := dir.EffectiveShortcuts()
			if len(scs) == 0 {
				continue
			}
			fmt.Printf("%s (%s):\n", dirName, alias)
			for _, s := range scs {
				fmt.Printf("  %s — %s\n", s.Cmd, s.Desc)
			}
			for _, svcName := range dir.ServiceOrder {
				svc := dir.Services[svcName]
				if svc == nil {
					continue
				}
				for _, s := range svc.Shortcuts {
					fmt.Printf("  %s — %s [%s]\n", s.Cmd, s.Desc, svcName)
				}
			}
		}
	},
}

func init() {
	runCmd.Flags().StringP("branch", "b", "", "Workspace branch (default: main)")
	runCmd.AddCommand(runListCmd)

	shortcuts := strings.Replace(runListCmd.UseLine(), "shortcuts", "shortcuts", 1)
	_ = shortcuts
}
