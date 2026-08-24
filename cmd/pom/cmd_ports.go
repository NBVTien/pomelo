package main

import (
	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/commands"
)

var portsWatch bool

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "Show the live port registry (each service's port + lifecycle state)",
	RunE: func(cmd *cobra.Command, args []string) error {
		commands.Ports(portsWatch)
		return nil
	},
}

func init() {
	portsCmd.Flags().BoolVarP(&portsWatch, "watch", "w", false, "refresh continuously")
}
