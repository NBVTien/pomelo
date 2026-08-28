package core

import (
	"encoding/json"
	"testing"
)

func TestReviewThreadsLifecycle(t *testing.T) {
	root := t.TempDir()
	s := &Server{WorkspaceRoot: root}

	add := s.ReviewThreadAdd(reviewThreadAddReq{
		Branch: "feat-x", IsMain: true, Repo: "api", Path: "a.go",
		Start: 10, End: 12, Body: "why here?", Author: "dat",
	})
	if ok, _ := add["ok"].(bool); !ok {
		t.Fatalf("add failed: %+v", add)
	}
	id, _ := add["id"].(string)
	if id == "" {
		t.Fatal("no thread id")
	}

	if r := s.ReviewThreadReply("feat-x", id, "because concurrency", "sam", true); !r["ok"].(bool) {
		t.Fatalf("reply failed: %+v", r)
	}
	if r := s.ReviewThreadResolve("feat-x", id, true, true); !r["ok"].(bool) {
		t.Fatalf("resolve failed: %+v", r)
	}

	var got struct {
		Threads []struct {
			ID       string
			Repo     string
			Path     string
			Start    int
			Resolved bool
			Comments []struct{ Author, Body string }
		}
	}
	if err := json.Unmarshal(s.ReviewThreads("feat-x", true), &got); err != nil {
		t.Fatalf("list decode: %v", err)
	}
	if len(got.Threads) != 1 {
		t.Fatalf("want 1 thread, got %d", len(got.Threads))
	}
	th := got.Threads[0]
	if th.ID != id || th.Repo != "api" || th.Path != "a.go" || th.Start != 10 || !th.Resolved {
		t.Fatalf("bad thread: %+v", th)
	}
	if len(th.Comments) != 2 || th.Comments[0].Author != "dat" || th.Comments[1].Body != "because concurrency" {
		t.Fatalf("bad comments: %+v", th.Comments)
	}

	// empty body rejected; missing thread rejected
	if s.ReviewThreadAdd(reviewThreadAddReq{Branch: "feat-x", Repo: "api", Path: "a.go", Body: "  "})["ok"].(bool) {
		t.Fatal("empty body should be rejected")
	}
	if s.ReviewThreadReply("feat-x", "nope", "x", "", true)["ok"].(bool) {
		t.Fatal("reply to missing thread should fail")
	}

	// list for a branch with no threads is still valid json
	if !json.Valid(s.ReviewThreads("other", true)) {
		t.Fatal("empty threads list must be valid json")
	}
}
