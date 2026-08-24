package main

import (
	"encoding/json"
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

var configExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Print the merged config (all pom.d fragments) — to share or back up",
	Long: `Prints the full config with every pom.d/*.yml fragment inlined, so you
can share it in one piece (e.g. paste it to get help) or back it up.
--redact scrubs secrets and environment URLs first.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		out, _ := cmd.Flags().GetString("out")
		redact, _ := cmd.Flags().GetBool("redact")
		return commands.ConfigExport(configPath, out, redact)
	},
}

var configImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Replace the active config with <file> (validates + backs up first)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		return commands.ConfigImport(configPath, args[0], force)
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect and reorganize the project config",
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the resolved config file path",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(configPath)
		return nil
	},
}

var configSplitCmd = &cobra.Command{
	Use:   "split",
	Short: "Split repos into pom.d/<repo>.yml",
	Long: `Moves each repo out of the single config file into its own
pom.d/<repo>.yml fragment, keeping everything else in the root. The original is
kept as <name>.bak. Pomelo auto-merges pom.d/*.yml on load, so behavior is
unchanged.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dry, _ := cmd.Flags().GetBool("dry-run")
		res, err := config.SplitToFragments(configPath, dry)
		if err != nil {
			return err
		}
		verb := "Wrote"
		if dry {
			verb = "Would write"
		}
		fmt.Printf("root:   %s\n", res.RootFile)
		for _, f := range res.Fragments {
			fmt.Printf("%s: %s\n", verb, f)
		}
		if !dry {
			fmt.Printf("backup: %s\n", res.BackupFile)
			fmt.Println("Done. Reload the dashboard (or restart) to pick up the split config.")
		} else {
			fmt.Println("(dry run — nothing written; re-run without --dry-run to apply)")
		}
		return nil
	},
}

var configLintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Report shared-service fields that just repeat a built-in default",
	Long: `Lists, per shared service, the fields whose value equals Pomelo's
well-known default (postgres/redis/minio/opensearch) — safe to delete with no
behavioral change. Trims config bloat: name the service and let defaults fill
image/ports/env/creds instead of spelling them out.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		red, err := config.LintRedundantDefaults(configPath)
		if err != nil {
			return err
		}
		clean := true
		if len(red) > 0 {
			clean = false
			names := make([]string, 0, len(red))
			for n := range red {
				names = append(names, n)
			}
			sort.Strings(names)
			total := 0
			for _, n := range names {
				fields := red[n]
				sort.Strings(fields)
				total += len(fields)
				fmt.Printf("%s: %s\n", n, strings.Join(fields, ", "))
			}
			fmt.Printf("\n%d field(s) across %d service(s) match a built-in default — safe to delete.\n\n", total, len(names))
		}

		legacy, err := config.LintLegacyTokens(configPath)
		if err != nil {
			return err
		}
		if len(legacy) > 0 {
			clean = false
			fmt.Printf("%sDEPRECATED tokens%s (dual-support ends at schema_version 2 — run `pom config normalize --write`):\n", commands.Bold, commands.NC)
			for _, lt := range legacy {
				fmt.Printf("  %s → %s  (%d use%s)\n", lt.Old, lt.New, lt.Count, plural(lt.Count))
			}
			fmt.Println()
		}

		depKeys, err := config.LintDeprecatedKeys(configPath)
		if err != nil {
			return err
		}
		if len(depKeys) > 0 {
			clean = false
			fmt.Printf("%sDeprecated keys%s:\n", commands.Bold, commands.NC)
			for _, m := range depKeys {
				fmt.Printf("  %s\n", m)
			}
			fmt.Println()
		}

		dups, err := config.AnalyzeDuplicateEnv(configPath)
		if err != nil {
			return err
		}
		if len(dups) > 0 {
			clean = false
			fmt.Printf("%sDuplicated env%s (same value in ≥2 repos — hoist into a preset):\n", commands.Bold, commands.NC)
			for _, d := range dups {
				fmt.Printf("  %s=%s  (%s)\n", d.Key, truncateVal(d.Value), strings.Join(d.Repos, ", "))
			}
			fmt.Println()
		}

		if inline := config.LintInlineCmdEnv(appConfig); len(inline) > 0 {
			clean = false
			fmt.Printf("%sInline env in cmd%s (move to `env:` for a bare command — compose/Procfile style):\n", commands.Bold, commands.NC)
			for _, ic := range inline {
				fmt.Printf("  %s: %s\n", ic.Service, strings.Join(ic.Keys, " "))
			}
		}

		if clean {
			fmt.Println("Config is lean — no redundant defaults, no deprecated tokens or keys.")
		}
		return nil
	},
}

func truncateVal(s string) string {
	if len(s) > 40 {
		return s[:37] + "..."
	}
	return s
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

var configNormalizeCmd = &cobra.Command{
	Use:   "normalize",
	Short: "Tidy the whole config: migrate tokens + group lifecycle + format (one shot)",
	Long: `Standardizes the config in one command: (1) rewrites deprecated
{{branch_safe}} tokens to the filter form, (2) moves each repo's ops commands
into a lifecycle: block, (3) formats every fragment (2-space indent, sorted env).
Each step keeps its own safety net (backup + verify + rollback). Dry-run by
default; --write applies everything.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		write, _ := cmd.Flags().GetBool("write")

		tokens, err := config.MigrateTokens(configPath, write)
		if err != nil {
			return fmt.Errorf("migrate-tokens: %w", err)
		}
		life, err := config.GroupLifecycle(configPath, write)
		if err != nil {
			return fmt.Errorf("group-lifecycle: %w", err)
		}
		cmdenv, err := config.ExtractCmdEnv(configPath, write)
		if err != nil {
			return fmt.Errorf("extract-cmd-env: %w", err)
		}
		fmtFiles := 0
		for _, f := range append([]string{configPath}, config.FragmentPaths(configDir())...) {
			out, changed, err := config.FormatFile(f, true)
			if err != nil {
				return fmt.Errorf("fmt %s: %w", f, err)
			}
			if changed {
				fmtFiles++
				if write {
					if err := os.WriteFile(f, out, 0o644); err != nil {
						return err
					}
				}
			}
		}

		tokenN := 0
		for _, n := range tokens {
			tokenN += n
		}
		lifeN := 0
		for _, n := range life {
			lifeN += n
		}
		verb := "would"
		if write {
			verb = "did"
		}
		fmt.Printf("  tokens:    %s migrate %d\n", verb, tokenN)
		fmt.Printf("  cmd-env:   %s bare %d service(s)\n", verb, len(cmdenv))
		fmt.Printf("  lifecycle: %s group %d field(s) across %d repo(s)\n", verb, lifeN, len(life))
		fmt.Printf("  format:    %s tidy %d file(s)\n", verb, fmtFiles)
		if tokenN == 0 && lifeN == 0 && fmtFiles == 0 && len(cmdenv) == 0 {
			fmt.Println("\nConfig already normalized.")
		} else if !write {
			fmt.Println("\nDry run. Re-run with --write to apply.")
		} else {
			fmt.Println("\nNormalized — reload the dashboard.")
		}
		return nil
	},
}

var configExtractCmdEnvCmd = &cobra.Command{
	Use:   "extract-cmd-env",
	Short: "Move leading KEY=VAL env off each service's cmd into env: (bare command)",
	Long: `Adopts the compose/Procfile convention: a service's command should be
bare, with env as data. Moves leading env-prefixes (RACK_ENV=x WEB_CONCURRENCY=0
…) out of each cmd into the service's env:. Conservative — skips any cmd with a
quoted/complex prefix and services with modes. Safe: backup + reload-verify +
rollback. Dry-run by default; --write to apply.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		write, _ := cmd.Flags().GetBool("write")
		plan, err := config.ExtractCmdEnv(configPath, write)
		if err != nil {
			return err
		}
		if len(plan) == 0 {
			fmt.Println("No inline env in any service cmd — already bare.")
			return nil
		}
		svcs := make([]string, 0, len(plan))
		for s := range plan {
			svcs = append(svcs, s)
		}
		sort.Strings(svcs)
		verb := "would move"
		if write {
			verb = "moved"
		}
		for _, s := range svcs {
			fmt.Printf("  %s: %s %s → env:\n", s, verb, strings.Join(plan[s], " "))
		}
		if !write {
			fmt.Println("\nDry run. Re-run with --write to apply.")
		} else {
			fmt.Println("\nDone — reload the dashboard.")
		}
		return nil
	},
}

var configGroupLifecycleCmd = &cobra.Command{
	Use:   "group-lifecycle",
	Short: "Move each repo's ops commands (setup/seed/copy/pre_delete/pre_start/shortcuts) into a lifecycle: block",
	Long: `Separates operational commands from identity: rewrites each repo's flat
setup/seed/copy/pre_delete/pre_start/shortcuts into a grouped lifecycle: block,
de-cluttering the spec. Both layouts load identically (dual-support), so this is
cosmetic. Safe: backs up + verifies effective lifecycle unchanged, rolls back on
mismatch. Dry-run by default; --write to apply.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		write, _ := cmd.Flags().GetBool("write")
		plan, err := config.GroupLifecycle(configPath, write)
		if err != nil {
			return err
		}
		if len(plan) == 0 {
			fmt.Println("No flat lifecycle fields to group — already tidy.")
			return nil
		}
		names := make([]string, 0, len(plan))
		for n := range plan {
			names = append(names, n)
		}
		sort.Strings(names)
		verb := "would move"
		if write {
			verb = "moved"
		}
		for _, n := range names {
			fmt.Printf("  %s: %s %d field(s) → lifecycle:\n", n, verb, plan[n])
		}
		if !write {
			fmt.Println("\nDry run. Re-run with --write to apply.")
		} else {
			fmt.Println("\nDone — reload the dashboard.")
		}
		return nil
	},
}

var configFmtCmd = &cobra.Command{
	Use:   "fmt",
	Short: "Tidy config files: canonical indent, and --sort-env to order env blocks",
	Long: `A gofmt/terraform-fmt for pom config. Re-emits the root config and every
pom.d fragment with consistent 2-space indentation (comments preserved). With
--sort-env it alpha-sorts each env: block so a big wall of vars becomes
scannable — env order is cosmetic. A semantic-identity guard refuses to write if
the data would change. Dry-run by default; pass --write to apply.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		write, _ := cmd.Flags().GetBool("write")
		sortEnv, _ := cmd.Flags().GetBool("sort-env")

		files := append([]string{configPath}, config.FragmentPaths(configDir())...)
		changedAny := false
		for _, f := range files {
			out, changed, err := config.FormatFile(f, sortEnv)
			if err != nil {
				return err
			}
			if !changed {
				continue
			}
			changedAny = true
			if write {
				if err := os.WriteFile(f, out, 0o644); err != nil {
					return err
				}
				fmt.Printf("  %sformatted%s %s\n", commands.Green, commands.NC, f)
			} else {
				fmt.Printf("  %swould format%s %s\n", commands.Dim, commands.NC, f)
			}
		}
		if !changedAny {
			fmt.Println("All config files are already tidy.")
		} else if !write {
			fmt.Println("\nDry run — re-run with --write to apply.")
		}
		return nil
	},
}

var configExtractPresetCmd = &cobra.Command{
	Use:   "extract-preset <KEY> [KEY...]",
	Short: "Hoist env duplicated across repos into a shared preset (composition, not copy-paste)",
	Long: `Turns a ` + "`config lint`" + ` duplication finding into action: move the given env
keys (identical across ≥2 repos) into one preset and add that preset to each
repo, removing the copies. Safe: backs up every file, then verifies each repo's
RESOLVED env is unchanged (the preset re-provides what was removed) and rolls
back on any mismatch. Dry-run by default; --write to apply.

  pom config extract-preset --into pg DATABASE_URL --write`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("into")
		write, _ := cmd.Flags().GetBool("write")
		if name == "" {
			return fmt.Errorf("--into <preset-name> is required")
		}
		plan, err := config.PlanExtractPreset(configPath, name, args)
		if err != nil {
			return err
		}
		fmt.Printf("%sPlan%s hoist into preset %q:\n", commands.Bold, commands.NC, plan.Preset)
		keys := make([]string, 0, len(plan.Keys))
		for k := range plan.Keys {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("  %s=%s\n", k, truncateVal(plan.Keys[k]))
		}
		fmt.Printf("  → added to + removed from: %s\n", strings.Join(plan.Repos, ", "))

		if !write {
			fmt.Println("\nDry run. Re-run with --write to apply (backed up + verified).")
			return nil
		}
		if err := config.ApplyExtractPreset(configPath, plan); err != nil {
			return err
		}
		fmt.Printf("\n%sDone%s — resolved env verified unchanged. Reload the dashboard.\n", commands.Green, commands.NC)
		return nil
	},
}

var configMigrateTokensCmd = &cobra.Command{
	Use:   "migrate-tokens",
	Short: "Rewrite deprecated {{branch_safe}} tokens to the modern {{branch|safe}} form",
	Long: `Rewrites {{branch_safe}} / {{branch_hash}} / {{branch_host}} to the filter
form {{branch|safe}} / |hash / |host across the root config and every pom.d
fragment. Both forms resolve identically, so this is a cosmetic cleanup toward
one obvious syntax. Dry-run by default; pass --write to apply.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		write, _ := cmd.Flags().GetBool("write")
		changes, err := config.MigrateTokens(configPath, write)
		if err != nil {
			return err
		}
		if len(changes) == 0 {
			fmt.Println("No deprecated branch tokens found — nothing to migrate.")
			return nil
		}
		files := make([]string, 0, len(changes))
		for f := range changes {
			files = append(files, f)
		}
		sort.Strings(files)
		total := 0
		verb := "would rewrite"
		if write {
			verb = "rewrote"
		}
		for _, f := range files {
			total += changes[f]
			fmt.Printf("  %s: %s %d token%s\n", f, verb, changes[f], plural(changes[f]))
		}
		if write {
			fmt.Printf("\nMigrated %d token(s). Reload the dashboard to pick up changes.\n", total)
		} else {
			fmt.Printf("\n%d token(s) across %d file(s). Re-run with --write to apply.\n", total, len(files))
		}
		return nil
	},
}

var (
	explainBranch string
	explainEnv    string
	explainOut    string
)

var configExplainCmd = &cobra.Command{
	Use:   "explain [repo]",
	Short: "Show what the config resolves to — shared services, ports, DBs — and WHERE each value comes from",
	Long: `Makes config resolution visible. Prints, for a branch + environment:
  - SHARED SERVICES: resolved host:port (and creds) each {{shared.NAME.*}} uses;
  - DATABASES: the per-branch DB names each repo's {{db.NAME}} resolves to.
  Pass repo/service to see one service's fully-resolved cmd, port, DBs and env.

Pass a repo (name or alias) to scope the databases section.

  pom config explain                 # default branch, local env
  pom config explain --env staging   # what switches to the staging remotes
  pom config explain api --branch proj-123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := appConfig
		branch := explainBranch
		if branch == "" {
			branch = cfg.GlobalDefaultBranch()
		}
		envLabel := explainEnv
		if envLabel == "" {
			envLabel = "local"
		}
		if explainOut != "json" {
			fmt.Printf("%sConfig%s  %s\n%sBranch%s %s   %sEnv%s %s\n\n",
				commands.Bold, commands.NC, configPath,
				commands.Bold, commands.NC, branch, commands.Bold, commands.NC, envLabel)
		}

		if len(args) > 0 && strings.Contains(args[0], "/") {
			repo, svc, _ := strings.Cut(args[0], "/")
			se, err := services.ExplainService(cfg, repo, svc, branch, explainEnv)
			if err != nil {
				return err
			}
			if explainOut == "json" {
				return printJSON(se)
			}
			return printServiceExplain(se)
		}

		if explainOut == "json" && len(args) == 0 {
			return printJSON(map[string]any{"branch": branch, "env": envLabel})
		}

		if len(cfg.SharedOrder) > 0 {
			fmt.Printf("%sSHARED SERVICES%s  {{shared.NAME.host}} {{shared.NAME.port}} {{shared.NAME.url}}\n", commands.Bold, commands.NC)
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintf(tw, "  NAME\tHOST:PORT\tCREDS\n")
			for _, name := range cfg.SharedOrder {
				def := cfg.SharedServices[name]
				host := cfg.SharedHost(name)
				port := services.SharedPort(name)
				creds := "-"
				if def != nil && def.DBUser != "" {
					creds = def.DBUser + ":" + def.DBPassword
				}
				fmt.Fprintf(tw, "  %s\t%s:%d\t%s\n", name, host, port, creds)
			}
			tw.Flush()
			fmt.Println()
		}

		repoFilter := ""
		if len(args) > 0 {
			repoFilter = args[0]
		}
		printed := false
		for _, n := range cfg.RepoOrder {
			d := cfg.Repos[n]
			if d == nil || len(d.Databases) == 0 {
				continue
			}
			if repoFilter != "" && n != repoFilter && d.Alias != repoFilter {
				continue
			}
			if !printed {
				fmt.Printf("%sDATABASES%s  {{db.NAME}} — per-branch, session-prefixed\n", commands.Bold, commands.NC)
				printed = true
			}
			fmt.Printf("  %s%s%s\n", commands.Dim, n, commands.NC)
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			names := make([]string, 0, len(d.Databases))
			for k := range d.Databases {
				names = append(names, k)
			}
			sort.Strings(names)
			for _, k := range names {
				fmt.Fprintf(tw, "    {{db.%s}}\t%s_%s\n", k, cfg.Session, services.ResolveBranchTokens(d.Databases[k], branch))
			}
			tw.Flush()
		}
		return nil
	},
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printServiceExplain(se *services.ServiceExplain) error {
	fmt.Printf("%s%s/%s%s  (alias %s)\n\n", commands.Bold, se.Repo, se.Service, commands.NC, se.Alias)
	fmt.Printf("  %scmd%s      %s\n", commands.Bold, commands.NC, se.Cmd)
	port := fmt.Sprintf("%d", se.Port)
	if se.Port == 0 {
		port = "- (no port allocated on this branch — not running)"
	}
	fmt.Printf("  %sport%s     %s\n", commands.Bold, commands.NC, port)
	fmt.Println()

	if len(se.Databases) > 0 {
		fmt.Printf("%sDATABASES%s\n", commands.Bold, commands.NC)
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		names := make([]string, 0, len(se.Databases))
		for k := range se.Databases {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			fmt.Fprintf(tw, "  {{db.%s}}\t%s\n", k, se.Databases[k])
		}
		tw.Flush()
		fmt.Println()
	}

	if len(se.Env) > 0 {
		fmt.Printf("%sEFFECTIVE ENV%s  (dir.env + svc.env, fully resolved)\n", commands.Bold, commands.NC)
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		for _, e := range se.Env {
			fmt.Fprintf(tw, "  %s\t%s\n", e.Key, dashIfEmpty(e.Value))
		}
		tw.Flush()
	}
	return nil
}

func init() {
	configExplainCmd.Flags().StringVar(&explainBranch, "branch", "", "branch to resolve for (default: config default branch)")
	configExplainCmd.Flags().StringVar(&explainEnv, "env", "", "environment/profile to resolve for (default: local)")
	configExplainCmd.Flags().StringVarP(&explainOut, "output", "o", "table", "output format: table | json")
	configMigrateTokensCmd.Flags().Bool("write", false, "apply the rewrite (default: dry run)")
	configFmtCmd.Flags().Bool("write", false, "apply formatting (default: dry run)")
	configFmtCmd.Flags().Bool("sort-env", false, "alpha-sort entries within each env: block")
	configExtractPresetCmd.Flags().String("into", "", "preset name to hoist the keys into (required)")
	configExtractPresetCmd.Flags().Bool("write", false, "apply the extraction (default: dry run)")
	configGroupLifecycleCmd.Flags().Bool("write", false, "apply the grouping (default: dry run)")
	configExtractCmdEnvCmd.Flags().Bool("write", false, "apply (default: dry run)")
	configNormalizeCmd.Flags().Bool("write", false, "apply all normalization (default: dry run)")
	configSplitCmd.Flags().Bool("dry-run", false, "Show what would happen without writing")
	configExportCmd.Flags().StringP("out", "o", "", "write to a file instead of stdout")
	configExportCmd.Flags().Bool("redact", false, "scrub secrets + environment URLs (safe to share)")
	configImportCmd.Flags().Bool("force", false, "move pom.d/ aside so the imported file is the single source")
	configCmd.AddCommand(configPathCmd, configSplitCmd, configExportCmd, configImportCmd, configLintCmd, configExplainCmd, configMigrateTokensCmd, configFmtCmd, configExtractPresetCmd, configGroupLifecycleCmd, configNormalizeCmd, configExtractCmdEnvCmd)
}
