package pipeline

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSavePipelineState_StatusByPosition(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	labels := []string{"a", "b", "c", "d"}
	SavePipelineState("feat/z", "ws-z", OpCreateWorkspace, labels, 2, "acme")
	defer ClearPipelineState("feat/z")

	data, err := os.ReadFile(pipelineStatePath("feat/z"))
	if err != nil {
		t.Fatalf("state not written: %v", err)
	}
	var st PipelineState
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatal(err)
	}
	if st.Operation != OpCreateWorkspace || st.Branch != "feat/z" || st.FailedStage != 2 {
		t.Fatalf("header mismatch: %+v", st)
	}
	want := []StageStatus{StatusCompleted, StatusCompleted, StatusFailed, StatusPending}
	if len(st.Stages) != len(want) {
		t.Fatalf("got %d stages, want %d", len(st.Stages), len(want))
	}
	for i, w := range want {
		if st.Stages[i].Status != w {
			t.Errorf("stage %d status = %q, want %q", i, st.Stages[i].Status, w)
		}
	}
	if st.Stages[2].Error != "acme" {
		t.Errorf("failed stage error = %q, want %q", st.Stages[2].Error, "acme")
	}
	if st.Stages[0].Error != "" {
		t.Errorf("completed stage should carry no error, got %q", st.Stages[0].Error)
	}
}

func TestClearPipelineState_Removes(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	SavePipelineState("b", "w", OpDeleteWorkspace, []string{"x"}, 0, "e")
	if _, err := os.Stat(pipelineStatePath("b")); err != nil {
		t.Fatalf("expected state file: %v", err)
	}
	ClearPipelineState("b")
	if _, err := os.Stat(pipelineStatePath("b")); !os.IsNotExist(err) {
		t.Errorf("state file should be gone, stat err = %v", err)
	}
}
