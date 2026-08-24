package main

import (
	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/commands"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check tools, services and config; point at fixes",
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.Doctor()
	},
}
