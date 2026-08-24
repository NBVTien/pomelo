package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sess.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLastAssistantText(t *testing.T) {
	p := writeTranscript(t,
		`{"type":"user","message":{"content":[{"type":"text","text":"do the thing"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"working on it"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Should I delete the old file?"}]}}`,
		`{"type":"system","message":{}}`,
	)
	if got := lastAssistantText(p); got != "Should I delete the old file?" {
		t.Fatalf("got %q", got)
	}
}

func TestLastAssistantTextSkipsToolOnly(t *testing.T) {
	p := writeTranscript(t,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"the question"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{}}]}}`,
	)
	if got := lastAssistantText(p); got != "the question" {
		t.Fatalf("tool-only message should not clobber text: %q", got)
	}
}

func TestLastAssistantTextMissingFile(t *testing.T) {
	if got := lastAssistantText(filepath.Join(t.TempDir(), "nope.jsonl")); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	if got := lastAssistantText(""); got != "" {
		t.Fatalf("expected empty for empty path, got %q", got)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes(strings.Repeat("ă", 500), 400); len([]rune(got)) != 401 {
		t.Fatalf("bad truncation length: %d", len([]rune(got)))
	}
}
