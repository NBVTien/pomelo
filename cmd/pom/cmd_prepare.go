package main

import (
	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/commands"
)

var prepareMainCmd = &cobra.Command{
	Use:   "prepare-main",
	Short: "Refresh main's golden source: reset DBs + migrate + seed",
	Long: "Rebuilds the MAIN workspace's databases (safe drop that terminates " +
		"connections first), runs each repo's migrations, then seeds. New " +
		"workspaces clone these DBs via CREATE DATABASE ... TEMPLATE.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.PrepareMain(appConfig, configDir())
	},
}
