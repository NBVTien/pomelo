package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/config"
	"github.com/pomelohq/pomelo/internal/core"
	"github.com/pomelohq/pomelo/internal/mcp"
	"github.com/pomelohq/pomelo/internal/services"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run an MCP server exposing this workspace's environment to an agent",
	Long: `Starts a Model Context Protocol server (stdio) that lets a coding agent
inspect and act on the workspace's real environment — ports, databases,
services, and pom.yml.

Portless: builds the /api handler in-process from the project's config
(found by walking up from the current directory), so it needs no running
dashboard. Launched automatically inside Claude windows pom spawns.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
	RunE: func(cmd *cobra.Command, args []string) error {
		branch, _ := cmd.Flags().GetString("branch")
		services.LoadLoginShellEnv()
		path, err := config.FindConfig()
		if err != nil {
			return err
		}
		cfg, err := config.Load(path)
		if err != nil {
			return err
		}
		dir := filepath.Dir(path)
		services.InitNetwork(dir, cfg.Session, cfg)
		services.SetSharedStable(cfg.Session)
		h := core.New("", cfg.Session, dir, cfg.GlobalDefaultBranch(), cfg).Handler()
		return mcp.Serve(os.Stdin, os.Stdout, "pomelo", version, mcp.ToolsHandler(h, branch))
	},
}

func init() {
	mcpCmd.Flags().String("branch", "", "Workspace branch this server is scoped to")
}
