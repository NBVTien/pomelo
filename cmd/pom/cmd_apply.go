package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/commands"
	"github.com/pomelohq/pomelo/internal/services"
)

var applyYes bool

var applyCmd = &cobra.Command{
	Use:   "apply [branch]",
	Short: "Converge workspace(s) to match config — create missing repo worktrees (never deletes)",
	Long: `Reconcile existing workspaces toward the config spec: any repo that config
declares (with worktree config) but a workspace is missing gets its worktree +
DBs + env created. Converge-only — apply NEVER deletes a workspace or repo.

Dry-run by default (shows the plan); pass --yes to execute. With no branch, plans
every workspace.

  pom apply                 # plan drift across all workspaces
  pom apply proj-123 --yes   # create proj-123's missing repos`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var targets []services.WorkspaceStatus
		if len(args) > 0 {
			st, ok := services.WorkspaceStatusFor(configDir(), appConfig, args[0])
			if !ok {
				return fmt.Errorf("no workspace %q", args[0])
			}
			targets = []services.WorkspaceStatus{*st}
		} else {
			targets = services.ScanWorkspaceStatus(configDir(), appConfig)
		}

		var drifted []services.WorkspaceStatus
		for _, w := range targets {
			if len(w.MissingRepos) > 0 {
				drifted = append(drifted, w)
			}
		}
		if len(drifted) == 0 {
			fmt.Println("All workspaces match config — nothing to apply.")
			return nil
		}

		fmt.Printf("%sPlan%s (converge-only, no deletes):\n", commands.Bold, commands.NC)
		for _, w := range drifted {
			fmt.Printf("  %s: + %s\n", w.Branch, strings.Join(w.MissingRepos, ", "))
		}

		if !applyYes {
			fmt.Printf("\nDry run. Re-run with --yes to create the missing worktrees.\n")
			return nil
		}

		fmt.Println()
		for _, w := range drifted {
			fmt.Printf("%s>>> converging %s%s\n", commands.Blue, w.Branch, commands.NC)
			if err := commands.WorkspaceCreate(appConfig, configPath, w.Branch, w.Branch, 1, strings.Join(w.MissingRepos, ","), "", false); err != nil {
				return fmt.Errorf("%s: %w", w.Branch, err)
			}
		}
		return nil
	},
}

func init() {
	applyCmd.Flags().BoolVar(&applyYes, "yes", false, "execute the plan (default: dry run)")
}
