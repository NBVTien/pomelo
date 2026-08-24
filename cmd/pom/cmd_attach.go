package main

import (
	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/commands"
)

var attachCmd = &cobra.Command{
	Use:   "attach [target]",
	Short: "Attach a terminal to a running service",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := ""
		if len(args) > 0 {
			target = args[0]
		}
		return commands.Attach(appConfig, target)
	},
}
