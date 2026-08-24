package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/commands"
	"github.com/pomelohq/pomelo/internal/services"
)

var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Free this machine's local footprint for the project (services, containers, disk)",
	Long: `Tears down the project's local runtime to reclaim memory and disk — useful
after working locally when you go back to driving a remote server. Code is
safe: it lives in git and on the server.

  pom release                # stop services + remove shared containers (frees RAM)
  pom release --disk         # also remove docker volumes (drops DB data — frees disk)
  pom release --worktrees    # also delete branch workspaces (keeps main; frees disk)
  pom release --disk --worktrees --yes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		disk, _ := cmd.Flags().GetBool("disk")
		worktrees, _ := cmd.Flags().GetBool("worktrees")
		yes, _ := cmd.Flags().GetBool("yes")
		if (disk || worktrees) && !yes {
			warn := "This removes docker volumes (DB data)"
			if worktrees {
				warn = "This removes branch workspaces and docker volumes"
			}
			fmt.Printf("%s for %s. Continue? [y/N] ", warn, appConfig.Session)
			line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			if s := strings.ToLower(strings.TrimSpace(line)); s != "y" && s != "yes" {
				fmt.Println("aborted")
				return nil
			}
		}

		fmt.Println("• stopping services…")
		_ = commands.Stop(appConfig, configPath, "")

		dir := configDir()
		composeFile := filepath.Join(dir, "docker-compose.shared.yml")
		if _, err := os.Stat(composeFile); err == nil {
			project := appConfig.Session + "-shared"
			downArgs := []string{"compose", "-f", composeFile, "-p", project, "down"}
			if disk {
				downArgs = append(downArgs, "-v")
			}
			fmt.Println("• removing shared containers…")
			_ = exec.Command("docker", downArgs...).Run()
		}

		if worktrees {
			removeBranchWorktrees(dir, appConfig.GlobalDefaultBranch())
		}
		fmt.Printf("Released local footprint for %s.\n", appConfig.Session)
		return nil
	},
}

func removeBranchWorktrees(dir, defaultBranch string) {
	mainDir := filepath.Join(dir, "workspace--"+services.BranchSafe(defaultBranch))
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "workspace--") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if p == mainDir {
			continue
		}
		fmt.Printf("• removing %s\n", e.Name())
		_ = os.RemoveAll(p)
	}
	if repos, _ := os.ReadDir(mainDir); repos != nil {
		for _, r := range repos {
			if r.IsDir() {
				_ = exec.Command("git", "-C", filepath.Join(mainDir, r.Name()), "worktree", "prune").Run()
			}
		}
	}
}

func init() {
	releaseCmd.Flags().Bool("disk", false, "Also remove docker volumes (drops DB data)")
	releaseCmd.Flags().Bool("worktrees", false, "Also delete branch workspaces (keeps main)")
	releaseCmd.Flags().Bool("yes", false, "Skip the confirmation prompt")
}
