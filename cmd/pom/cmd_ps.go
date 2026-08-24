package main

import (
	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/commands"
)

var psWatch bool

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "CPU/RAM of every process pom spawned (pty holders + services)",
	RunE: func(cmd *cobra.Command, args []string) error {
		commands.Ps(psWatch)
		return nil
	},
}

func init() {
	psCmd.Flags().BoolVarP(&psWatch, "watch", "w", false, "refresh continuously")
}
