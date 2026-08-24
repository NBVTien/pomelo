package core

import (
	"testing"

	"github.com/pomelohq/pomelo/internal/plugin"
)

func TestFeaturesRegistry(t *testing.T) {
	s := New(":0", "proj", "", "main", nil)
	feats := s.features()

	want := map[string]bool{
		"jira": true, "pr": true,
		"claude": true, "version": true, "git": true, "activity": true,
	}
	if len(feats) != len(want) {
		t.Fatalf("features() returned %d features, want %d", len(feats), len(want))
	}
	seen := map[string]bool{}
	for _, f := range feats {
		name := f.Name()
		if seen[name] {
			t.Errorf("duplicate feature name %q", name)
		}
		seen[name] = true
		if !want[name] {
			t.Errorf("unexpected feature %q — update this test if intended", name)
		}
		if _, ok := f.(plugin.HTTPProvider); !ok {
			t.Errorf("feature %q does not implement HTTPProvider", name)
		}
	}
}
