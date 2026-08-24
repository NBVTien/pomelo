package jira

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestKeyForBranch(t *testing.T) {
	cases := map[string]string{
		"task-725-add-chat": "TASK-725",
		"abc-123":           "ABC-123",
		"fix/login":         "",
		"725-no-project":    "",
		"":                  "",
	}
	for in, want := range cases {
		if got := KeyForBranch(in); got != want {
			t.Errorf("KeyForBranch(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestADFText(t *testing.T) {
	adf := `{"type":"doc","content":[
		{"type":"paragraph","content":[{"type":"text","text":"Hello "},{"type":"text","text":"world"}]},
		{"type":"paragraph","content":[{"type":"text","text":"second"}]}
	]}`
	got := strings.TrimSpace(ADFText(json.RawMessage(adf)))
	if !strings.Contains(got, "Hello world") || !strings.Contains(got, "second") {
		t.Errorf("ADFText = %q", got)
	}
	if ADFText(nil) != "" {
		t.Error("ADFText(nil) should be empty")
	}
}

func TestResolveUnconfigured(t *testing.T) {
	if Resolve(nil) != nil {
		t.Error("Resolve(nil) must be nil")
	}
}
