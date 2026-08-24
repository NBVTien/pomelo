package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/commands"
	"github.com/pomelohq/pomelo/internal/config"
	"github.com/pomelohq/pomelo/internal/services"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage repo environment variables (list/get/set/unset)",
}

var envBranch, envProfile string

var envLsCmd = &cobra.Command{
	Use:   "ls <repo>",
	Short: "List a repo's env, fully resolved (which value each var actually gets)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, svc := splitRepoSvc(args[0])
		if svc == "" {
			svc = firstService(repo)
		}
		branch := envBranch
		if branch == "" {
			branch = appConfig.GlobalDefaultBranch()
		}
		se, err := services.ExplainService(appConfig, repo, svc, branch, envProfile)
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		for _, e := range se.Env {
			fmt.Fprintf(tw, "%s\t%s\n", e.Key, dashIfEmpty(e.Value))
		}
		return tw.Flush()
	},
}

var envGetCmd = &cobra.Command{
	Use:   "get <repo> <KEY>",
	Short: "Print one env var's resolved value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, svc := splitRepoSvc(args[0])
		if svc == "" {
			svc = firstService(repo)
		}
		branch := envBranch
		if branch == "" {
			branch = appConfig.GlobalDefaultBranch()
		}
		se, err := services.ExplainService(appConfig, repo, svc, branch, envProfile)
		if err != nil {
			return err
		}
		for _, e := range se.Env {
			if e.Key == args[1] {
				fmt.Println(e.Value)
				return nil
			}
		}
		return fmt.Errorf("no env var %q for %s", args[1], repo)
	},
}

var envSetCmd = &cobra.Command{
	Use:   "set <repo> KEY=VALUE [KEY=VALUE...]",
	Short: "Set env var(s) in a repo's config",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo := args[0]
		kv := map[string]string{}
		for _, pair := range args[1:] {
			k, v, ok := strings.Cut(pair, "=")
			if !ok {
				return fmt.Errorf("expected KEY=VALUE, got %q", pair)
			}
			kv[k] = v
		}
		if err := config.SetRepoEnv(configPath, repo, kv); err != nil {
			return err
		}
		keys := make([]string, 0, len(kv))
		for k := range kv {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Printf("%sSet%s %s on %s. Reload the dashboard.\n", commands.Green, commands.NC, strings.Join(keys, ", "), repo)
		return nil
	},
}

var envUnsetCmd = &cobra.Command{
	Use:   "unset <repo> KEY [KEY...]",
	Short: "Remove env var(s) from a repo's config",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo := args[0]
		if err := config.UnsetRepoEnv(configPath, repo, args[1:]); err != nil {
			return err
		}
		fmt.Printf("%sUnset%s %s on %s. Reload the dashboard.\n", commands.Green, commands.NC, strings.Join(args[1:], ", "), repo)
		return nil
	},
}

func splitRepoSvc(s string) (string, string) {
	r, v, _ := strings.Cut(s, "/")
	return r, v
}

func firstService(repo string) string {
	d := appConfig.Repos[repo]
	if d == nil {
		for _, dd := range appConfig.Repos {
			if dd.Alias == repo {
				d = dd
				break
			}
		}
	}
	if d != nil && len(d.ServiceOrder) > 0 {
		return d.ServiceOrder[0]
	}
	return ""
}

func init() {
	for _, c := range []*cobra.Command{envLsCmd, envGetCmd} {
		c.Flags().StringVar(&envBranch, "branch", "", "branch to resolve for (default: config default branch)")
		c.Flags().StringVar(&envProfile, "env", "", "environment/profile to resolve for (default: local)")
	}
	envCmd.AddCommand(envLsCmd, envGetCmd, envSetCmd, envUnsetCmd)
}
