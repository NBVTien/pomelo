package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatFile_SortEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pom.yml")
	in := "session: t\nrepos:\n  api:\n    env:\n      ZED: '1'\n      ALPHA: '2'\n      MID: '3'\n"
	os.WriteFile(path, []byte(in), 0o644)

	out, changed, err := FormatFile(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected changes from sorting")
	}
	s := string(out)
	if strings.Index(s, "ALPHA") > strings.Index(s, "MID") || strings.Index(s, "MID") > strings.Index(s, "ZED") {
		t.Errorf("env not sorted:\n%s", s)
	}
	if !strings.Contains(s, "api:") || !strings.Contains(s, "env:") {
		t.Errorf("structure lost:\n%s", s)
	}
}

func TestFormatFile_SemanticIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pom.yml")
	in := "session: t\nrepos:\n  web: {}\n  api:\n    env:\n      B: '1'\n      A: '2'\n"
	os.WriteFile(path, []byte(in), 0o644)

	out, _, err := FormatFile(path, true)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Index(s, "web:") > strings.Index(s, "api:") {
		t.Errorf("repo order changed (should be preserved):\n%s", s)
	}
	if !sameData([]byte(in), out) {
		t.Error("formatted data differs from input")
	}
}
