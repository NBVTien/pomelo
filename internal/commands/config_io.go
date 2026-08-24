package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pomelohq/pomelo/internal/config"
)

func ConfigExport(configPath, out string, redact bool) error {
	data, split, err := config.MergedYAML(configPath)
	if err != nil {
		return err
	}
	if redact {
		if data, err = config.RedactYAML(data); err != nil {
			return err
		}
	}
	if out == "" {
		_, err = os.Stdout.Write(data)
		return err
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("%s>>>%s exported config → %s\n", Green, NC, out)
	if split {
		fmt.Printf("%s(merged from pom.d/ fragments)%s\n", Dim, NC)
	}
	if redact {
		fmt.Printf("%ssecrets + environment URLs redacted — safe to share%s\n", Dim, NC)
	}
	return nil
}

func ConfigImport(configPath, in string, force bool) error {
	raw, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	tmp := filepath.Join(os.TempDir(), "pom-import-check.yml")
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	defer os.Remove(tmp)
	if _, err := config.Load(tmp); err != nil {
		return fmt.Errorf("refusing to import — the config does not validate:\n%w", err)
	}

	pomd := filepath.Join(filepath.Dir(configPath), "pom.d")
	if dirExists(pomd) && !force {
		return fmt.Errorf("pom.d/ exists — its fragments would merge on top of the imported file.\n" +
			"Re-run with --force to move pom.d aside (→ pom.d.bak) and import as the single source")
	}
	if cur, err := os.ReadFile(configPath); err == nil {
		_ = os.WriteFile(configPath+".bak", cur, 0o644)
	}
	if force && dirExists(pomd) {
		if err := os.Rename(pomd, pomd+".bak"); err != nil {
			return fmt.Errorf("move pom.d aside: %w", err)
		}
	}
	if err := os.WriteFile(configPath, raw, 0o644); err != nil {
		return err
	}
	fmt.Printf("%s>>>%s imported config → %s (backup: %s.bak)\n",
		Green, NC, configPath, filepath.Base(configPath))
	fmt.Println("Reload the dashboard (or restart) to pick up the change.")
	return nil
}
