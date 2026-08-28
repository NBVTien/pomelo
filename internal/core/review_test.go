package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReviewGetAndFilePeek(t *testing.T) {
	root := t.TempDir()
	// A workspace review at the project root, keyed by branch.
	revDir := filepath.Join(root, ".pom", "reviews")
	if err := os.MkdirAll(revDir, 0o755); err != nil {
		t.Fatal(err)
	}
	art := `{"exists":true,"id":"r1","title":"T","doc":"see [x](pom://code?repo=api&path=a.go&start=1&end=2)","anchors":[{"id":"a1","repo":"api","path":"a.go","start":1,"end":2}]}`
	if err := os.WriteFile(filepath.Join(revDir, "feat-x.json"), []byte(art), 0o644); err != nil {
		t.Fatal(err)
	}
	// A repo worktree file for the peek (main workspace: repos sit at the root).
	if err := os.MkdirAll(filepath.Join(root, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "api", "a.go"), []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Server{WorkspaceRoot: root}

	// review_get
	var rev struct {
		Exists  bool `json:"exists"`
		Anchors []struct{ Repo, Path string } `json:"anchors"`
	}
	if err := json.Unmarshal(s.ReviewGet("feat-x", true), &rev); err != nil {
		t.Fatalf("review decode: %v", err)
	}
	if !rev.Exists || len(rev.Anchors) != 1 || rev.Anchors[0].Repo != "api" {
		t.Fatalf("bad review: %+v", rev)
	}
	if !json.Valid(s.ReviewGet("nope", true)) {
		t.Fatal("missing review should still be valid json")
	}

	// file_peek
	var pk struct{ Content, Error string }
	if err := json.Unmarshal(s.FilePeek("feat-x", "api", "a.go", true), &pk); err != nil {
		t.Fatalf("peek decode: %v", err)
	}
	if pk.Error != "" || pk.Content != "line1\nline2\nline3\n" {
		t.Fatalf("bad peek: %+v", pk)
	}

	// path traversal is rejected
	var bad struct{ Error string }
	_ = json.Unmarshal(s.FilePeek("feat-x", "api", "../../etc/passwd", true), &bad)
	if bad.Error == "" {
		t.Fatal("traversal path should be rejected")
	}
}
