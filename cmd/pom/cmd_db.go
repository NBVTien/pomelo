package main

import (
	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/commands"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database management",
}

var dbResetCmd = &cobra.Command{
	Use:   "reset <branch>",
	Short: "Drop and recreate databases for a workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.DBReset(appConfig, configDir(), args[0])
	},
}

var dbCreateCmd = &cobra.Command{
	Use:   "create <branch>",
	Short: "Create databases for a workspace (skip existing)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.DBCreate(appConfig, configDir(), args[0])
	},
}

var dbDropCmd = &cobra.Command{
	Use:   "drop <branch>",
	Short: "Drop databases for a workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.DBDrop(appConfig, configDir(), args[0])
	},
}

var dbCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove orphan databases not belonging to any workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		return commands.DBClean(appConfig, configDir(), dryRun)
	},
}

func init() {
	dbCleanCmd.Flags().Bool("dry-run", false, "List orphans without dropping")
	dbCmd.AddCommand(dbResetCmd, dbCreateCmd, dbDropCmd, dbCleanCmd)
}
