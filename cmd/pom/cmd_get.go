package main

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/commands"
	"github.com/pomelohq/pomelo/internal/services"
)

var getOut string

var getCmd = &cobra.Command{
	Use:     "get <workspaces>",
	Aliases: []string{"g"},
	Short:   "List resources and their observed status (kubectl-style)",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "workspaces", "workspace", "ws":
			return getWorkspaces()
		default:
			return fmt.Errorf("unknown resource %q (try: workspaces)", args[0])
		}
	},
}

func getWorkspaces() error {
	ws := services.ScanWorkspaceStatus(configDir(), appConfig)
	sort.Slice(ws, func(i, j int) bool { return ws[i].Branch < ws[j].Branch })

	if getOut == "json" {
		return printJSON(ws)
	}
	if len(ws) == 0 {
		fmt.Println("No workspaces. Create one with `pom workspace create`.")
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 3, ' ', 0)
	fmt.Fprintf(tw, "NAME\tREADY\tPHASE\tAGE\n")
	for _, w := range ws {
		name := w.Branch
		if w.IsMain {
			name += " (main)"
		}
		fmt.Fprintf(tw, "%s\t%d/%d\t%s\t%s\n", name, w.Ready, w.Total, w.Phase, ageOf(w.Path))
	}
	return tw.Flush()
}

var describeCmd = &cobra.Command{
	Use:   "describe workspace <branch>",
	Short: "Show a resource's spec and observed status (kubectl-style)",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "workspace", "workspaces", "ws":
			return describeWorkspace(args[1])
		default:
			return fmt.Errorf("unknown resource %q (try: workspace <branch>)", args[0])
		}
	},
}

func describeWorkspace(branch string) error {
	st, ok := services.WorkspaceStatusFor(configDir(), appConfig, branch)
	if !ok {
		return fmt.Errorf("no workspace %q", branch)
	}
	if getOut == "json" {
		return printJSON(st)
	}
	fmt.Printf("%sWorkspace%s  %s\n", commands.Bold, commands.NC, st.Branch)
	fmt.Printf("  phase   %s (%d/%d services up)\n", st.Phase, st.Ready, st.Total)
	fmt.Printf("  path    %s\n", st.Path)
	fmt.Printf("  age     %s\n", ageOf(st.Path))
	if len(st.MissingRepos) > 0 {
		fmt.Printf("  %sdrift%s   %d repo(s) in config with no worktree here: %v\n", commands.Yellow, commands.NC, len(st.MissingRepos), st.MissingRepos)
		fmt.Printf("          (`pom apply` would create them — not yet built)\n")
	}
	fmt.Println()

	if len(st.Services) == 0 {
		return nil
	}
	fmt.Printf("%sSERVICES%s\n", commands.Bold, commands.NC)
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 3, ' ', 0)
	fmt.Fprintf(tw, "  REPO/SERVICE\tSTATUS\tPORT\n")
	for _, s := range st.Services {
		status := "stopped"
		if s.Up {
			status = "up"
		}
		port := "-"
		if s.Port > 0 {
			port = fmt.Sprintf("%d", s.Port)
		}
		fmt.Fprintf(tw, "  %s/%s\t%s\t%s\n", s.Repo, s.Name, status, port)
	}
	return tw.Flush()
}

func ageOf(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return "-"
	}
	d := time.Since(fi.ModTime())
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func init() {
	getCmd.Flags().StringVarP(&getOut, "output", "o", "table", "output format: table | json")
	describeCmd.Flags().StringVarP(&getOut, "output", "o", "table", "output format: table | json")
}
