package main

import (
	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/commands"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Initial setup (gitignore, port pool)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.Setup()
	},
}
