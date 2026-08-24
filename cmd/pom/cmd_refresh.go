package main

import (
	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/commands"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Stop this project's running services (free their ports)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.Refresh(appConfig)
	},
}
