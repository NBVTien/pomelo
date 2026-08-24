package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLifecycleBlock_EqualsFlat(t *testing.T) {
	dir := t.TempDir()

	flat := filepath.Join(dir, "flat.yml")
	os.WriteFile(flat, []byte(
		"session: t\nrepos:\n  api:\n    setup: [bundle install]\n    seed: [rake seed]\n    pre_start: source .env\n"), 0o644)

	block := filepath.Join(dir, "block.yml")
	os.WriteFile(block, []byte(
		"session: t\nrepos:\n  api:\n    lifecycle:\n      setup: [bundle install]\n      seed: [rake seed]\n      pre_start: source .env\n"), 0o644)

	a, err := Load(flat)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Load(block)
	if err != nil {
		t.Fatal(err)
	}
	da, db := a.Repos["api"], b.Repos["api"]
	if len(db.Setup) != 1 || db.Setup[0] != "bundle install" {
		t.Fatalf("block setup = %v, want [bundle install]", db.Setup)
	}
	if da.Setup[0] != db.Setup[0] || da.Seed[0] != db.Seed[0] || da.PreStart != db.PreStart {
		t.Errorf("block != flat: flat=%+v block setup=%v seed=%v prestart=%q", da, db.Setup, db.Seed, db.PreStart)
	}
}

func TestLifecycleBlock_WinsOverFlat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pom.yml")
	os.WriteFile(path, []byte(
		"session: t\nrepos:\n  api:\n    setup: [old]\n    lifecycle:\n      setup: [new]\n"), 0o644)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Repos["api"].Setup; len(got) != 1 || got[0] != "new" {
		t.Errorf("setup = %v, want [new] (block wins)", got)
	}
}
