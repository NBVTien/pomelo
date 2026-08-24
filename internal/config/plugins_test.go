package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

type demoPlugin struct {
	DB      string   `yaml:"db"`
	Tables  []string `yaml:"tables"`
	Enabled bool     `yaml:"enabled"`
}

func TestDecodePlugin_PresentAbsent(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(`
plugins:
  demo:
    db: demodb
    tables: [a, b]
    enabled: true
`), &cfg); err != nil {
		t.Fatal(err)
	}

	got, err := DecodePlugin[demoPlugin](cfg.Plugins, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.DB != "demodb" || len(got.Tables) != 2 || !got.Enabled {
		t.Fatalf("decoded = %+v, want {demodb [a b] true}", got)
	}

	missing, err := DecodePlugin[demoPlugin](cfg.Plugins, "nope")
	if err != nil || missing != nil {
		t.Fatalf("absent plugin = (%v, %v), want (nil, nil)", missing, err)
	}
}

func TestDecodePlugin_NilMap(t *testing.T) {
	got, err := DecodePlugin[demoPlugin](nil, "demo")
	if err != nil || got != nil {
		t.Fatalf("nil blocks = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestDirPluginsRoundtrip(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(`
repos:
  api:
    plugins:
      demo: { db: apidb }
`), &cfg); err != nil {
		t.Fatal(err)
	}
	dir := cfg.Repos["api"]
	if dir == nil {
		t.Fatal("repo api missing")
	}
	got, err := DecodePlugin[demoPlugin](dir.Plugins, "demo")
	if err != nil || got == nil || got.DB != "apidb" {
		t.Fatalf("per-repo decode = (%+v, %v), want db=apidb", got, err)
	}
}
