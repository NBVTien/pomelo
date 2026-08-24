package main

import (
	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/commands"
)

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Scaffold a new Pomelo project from the git repo you're in",
	Long: `Creates a Pomelo project from the current git repo: clones it (with your
uncommitted changes) into a fresh project under the sessions root, detects a
dev command, writes a working pom.yml, and registers the session. Your current
repo is left untouched — cd into the new project and run pom.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) == 1 {
			name = args[0]
		}
		useClaude, _ := cmd.Flags().GetBool("claude")
		return commands.Init(name, useClaude)
	},
}

func init() {
	initCmd.Flags().Bool("claude", false, "After scaffolding, let claude tailor pom.yml with you (interactive)")
}
