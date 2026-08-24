package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pomelohq/pomelo/internal/config"
	"github.com/pomelohq/pomelo/internal/services"
)

func DBReset(cfg *config.Config, configDir, workspaceBranch string) error {

	type dbEntry struct {
		repo, dbName string
		port         uint16
		user, pw     string
	}
	var dbs []dbEntry

	wsDir := filepath.Join(configDir, "workspace--"+workspaceBranch)

	for dirName, dir := range cfg.Repos {
		if !dir.HasWorktreeConfig() || len(dir.Databases) == 0 {
			continue
		}
		if !services.DirExists(filepath.Join(wsDir, dirName)) {
			continue
		}

		pgSvc := FindPGService(cfg)
		pgPort := uint16(services.SharedPort("postgres"))
		if pgPort == 0 {
			pgPort = 5432
		}
		pgUser, pgPw := "postgres", "postgres"
		if pgSvc != nil {
			if pgSvc.DBUser != "" {
				pgUser = pgSvc.DBUser
			}
			if pgSvc.DBPassword != "" {
				pgPw = pgSvc.DBPassword
			}
		}

		for _, sref := range dir.SharedSvcRefs {
			if sref.DBName != "" {
				dbs = append(dbs, dbEntry{dirName, services.ResolveBranchTokens(sref.DBName, workspaceBranch), pgPort, pgUser, pgPw})
			}
		}
		for _, dbTpl := range dir.Databases {
			dbs = append(dbs, dbEntry{dirName, cfg.Session + "_" + services.ResolveBranchTokens(dbTpl, workspaceBranch), pgPort, pgUser, pgPw})
		}
	}

	if len(dbs) == 0 {
		fmt.Printf("%sNo databases found for workspace '%s'%s\n", Yellow, workspaceBranch, NC)
		return nil
	}

	fmt.Printf("%sResetting databases for workspace '%s':%s\n", Bold, workspaceBranch, NC)
	for _, db := range dbs {
		fmt.Printf("  %s: %s\n", db.repo, db.dbName)
	}
	fmt.Println()

	var dbNames []string
	for _, db := range dbs {
		dbNames = append(dbNames, db.dbName)
	}

	host := "localhost"
	if pgSvc := FindPGService(cfg); pgSvc != nil && pgSvc.Host != "" {
		host = pgSvc.Host
	}
	port, user, pw := dbs[0].port, dbs[0].user, dbs[0].pw

	fmt.Printf("%s>>>%s dropping %d databases...", Blue, NC, len(dbNames))
	if services.DropSharedDBsBatch(host, port, dbNames, user, pw) {
		fmt.Printf(" %sok%s\n", Green, NC)
	} else {
		fmt.Printf(" %ssome failed%s\n", Yellow, NC)
	}

	fmt.Printf("%s>>>%s creating %d databases...", Blue, NC, len(dbNames))
	services.CreateSharedDBsBatch(host, port, dbNames, user, pw)
	fmt.Printf(" %sok%s\n", Green, NC)

	fmt.Printf("\n%sDatabase reset complete for workspace '%s'.%s\n", Green, workspaceBranch, NC)
	fmt.Println("Run migrations to restore schema (e.g. via TUI shortcuts).")
	return nil
}

func DBCreate(cfg *config.Config, configDir, workspaceBranch string) error {
	dbNames, host, port, user, pw := resolveDBs(cfg, configDir, workspaceBranch)
	if len(dbNames) == 0 {
		fmt.Printf("%sNo databases found for workspace '%s'%s\n", Yellow, workspaceBranch, NC)
		return nil
	}
	fmt.Printf("%sCreating databases for workspace '%s':%s\n", Bold, workspaceBranch, NC)
	for _, db := range dbNames {
		fmt.Printf("  %s\n", db)
	}
	fmt.Println()
	fmt.Printf("%s>>>%s creating %d databases...", Blue, NC, len(dbNames))
	services.CreateSharedDBsBatch(host, port, dbNames, user, pw)
	fmt.Printf(" %sok%s\n", Green, NC)
	return nil
}

func DBDrop(cfg *config.Config, configDir, workspaceBranch string) error {
	dbNames, host, port, user, pw := resolveDBs(cfg, configDir, workspaceBranch)
	if len(dbNames) == 0 {
		fmt.Printf("%sNo databases found for workspace '%s'%s\n", Yellow, workspaceBranch, NC)
		return nil
	}
	fmt.Printf("%sDropping databases for workspace '%s':%s\n", Bold, workspaceBranch, NC)
	for _, db := range dbNames {
		fmt.Printf("  %s\n", db)
	}
	fmt.Println()
	fmt.Printf("%s>>>%s dropping %d databases...", Blue, NC, len(dbNames))
	if services.DropSharedDBsBatch(host, port, dbNames, user, pw) {
		fmt.Printf(" %sok%s\n", Green, NC)
	} else {
		fmt.Printf(" %ssome failed%s\n", Yellow, NC)
	}
	return nil
}

func DBClean(cfg *config.Config, configDir string, dryRun bool) error {
	host, port, user, pw := pgConnInfo(cfg)

	allDBs := services.ListSharedDBs(host, port, user, pw)
	if len(allDBs) == 0 {
		fmt.Printf("%sNo databases found in PostgreSQL%s\n", Yellow, NC)
		return nil
	}

	expected := allExpectedDBs(cfg, configDir)

	var orphans []string
	for _, db := range allDBs {
		if !expected[db] {
			orphans = append(orphans, db)
		}
	}

	if len(orphans) == 0 {
		fmt.Printf("%sNo orphan databases found%s (%d expected, %d total)\n", Green, NC, len(expected), len(allDBs))
		return nil
	}

	sort.Strings(orphans)
	fmt.Printf("%sOrphan databases (%d):%s\n", Bold, len(orphans), NC)
	for _, db := range orphans {
		fmt.Printf("  %s\n", db)
	}
	fmt.Println()

	if dryRun {
		fmt.Printf("%s(dry run — no databases dropped)%s\n", Dim, NC)
		return nil
	}

	fmt.Printf("Drop %d orphan databases? [y/N] ", len(orphans))
	var answer string
	fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" {
		fmt.Println("Cancelled.")
		return nil
	}

	fmt.Printf("%s>>>%s dropping %d orphan databases...", Blue, NC, len(orphans))
	if services.DropSharedDBsBatch(host, port, orphans, user, pw) {
		fmt.Printf(" %sok%s\n", Green, NC)
	} else {
		fmt.Printf(" %ssome failed%s\n", Yellow, NC)
	}
	return nil
}

func resolveDBs(cfg *config.Config, configDir, workspaceBranch string) (dbNames []string, host string, port uint16, user, pw string) {
	wsDir := filepath.Join(configDir, "workspace--"+workspaceBranch)

	for dirName, dir := range cfg.Repos {
		if !dir.HasWorktreeConfig() || len(dir.Databases) == 0 {
			continue
		}
		if !services.DirExists(filepath.Join(wsDir, dirName)) {
			continue
		}
		for _, sref := range dir.SharedSvcRefs {
			if sref.DBName != "" {
				dbNames = append(dbNames, services.ResolveBranchTokens(sref.DBName, workspaceBranch))
			}
		}
		for _, dbTpl := range dir.Databases {
			dbNames = append(dbNames, cfg.Session+"_"+services.ResolveBranchTokens(dbTpl, workspaceBranch))
		}
	}

	host, port, user, pw = pgConnInfo(cfg)
	return
}

const pgNameLimit = 63

func truncatePGName(name string) string {
	if len(name) > pgNameLimit {
		return name[:pgNameLimit]
	}
	return name
}

func allExpectedDBs(cfg *config.Config, configDir string) map[string]bool {
	expected := make(map[string]bool)

	entries, _ := os.ReadDir(configDir)
	for _, e := range entries {
		branch, ok := strings.CutPrefix(e.Name(), "workspace--")
		if !ok {
			continue
		}
		wsDir := filepath.Join(configDir, e.Name())

		for dirName, dir := range cfg.Repos {
			if !dir.HasWorktreeConfig() || len(dir.Databases) == 0 {
				continue
			}
			if !services.DirExists(filepath.Join(wsDir, dirName)) {
				continue
			}

			for _, sref := range dir.SharedSvcRefs {
				if sref.DBName != "" {
					expected[truncatePGName(services.ResolveBranchTokens(sref.DBName, branch))] = true
				}
			}
			for _, dbTpl := range dir.Databases {
				expected[truncatePGName(cfg.Session+"_"+services.ResolveBranchTokens(dbTpl, branch))] = true
			}
		}
	}
	return expected
}

func pgConnInfo(cfg *config.Config) (host string, port uint16, user, pw string) {
	host = "localhost"
	port = uint16(services.SharedPort("postgres"))
	if port == 0 {
		port = 5432
	}
	user, pw = "postgres", "postgres"
	if pgSvc := FindPGService(cfg); pgSvc != nil {
		if pgSvc.Host != "" {
			host = pgSvc.Host
		}
		if pgSvc.DBUser != "" {
			user = pgSvc.DBUser
		}
		if pgSvc.DBPassword != "" {
			pw = pgSvc.DBPassword
		}
	}
	return
}
