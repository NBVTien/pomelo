package core

import "testing"

func TestCleanLabel(t *testing.T) {
	cases := map[string]string{
		"PROJ-1069 map partner custom fields": "proj-1069-map-partner-custom-fields",
		"\"proj-855 fix send\"":               "proj-855-fix-send",
		"PROJ-1043 topics\nignored":           "proj-1043-topics",
		"  Prospect  Topics  ":                "prospect-topics",
	}
	for in, want := range cases {
		if got := cleanLabel(in); got != want {
			t.Errorf("cleanLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
