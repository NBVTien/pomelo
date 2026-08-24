package pipeline

import "testing"

func allSkipped(n int) map[int]bool {
	m := make(map[int]bool, n)
	for i := 0; i < n; i++ {
		m[i] = true
	}
	return m
}

func drain(run func(chan<- Event)) []Event {
	ch := make(chan Event, 64)
	run(ch)
	close(ch)
	var evs []Event
	for e := range ch {
		evs = append(evs, e)
	}
	return evs
}

func TestRunCreatePipeline_AllSkipped(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	ctx := &CreateContext{Branch: "feat/x", WorkspaceName: "ws-x", SkipStages: allSkipped(len(AllCreateStages))}
	evs := drain(func(ch chan<- Event) { RunCreatePipeline(ctx, ch) })

	if len(evs) != len(AllCreateStages)+1 {
		t.Fatalf("got %d events, want %d", len(evs), len(AllCreateStages)+1)
	}
	for i := 0; i < len(AllCreateStages); i++ {
		if evs[i].Type != EventStageSkipped || evs[i].Index != i {
			t.Errorf("event %d = %+v, want Skipped index %d", i, evs[i], i)
		}
	}
	if last := evs[len(evs)-1]; last.Type != EventPipelineCompleted || last.Branch != "feat/x" {
		t.Errorf("final event = %+v, want PipelineCompleted for feat/x", last)
	}
}

func TestRunDeletePipeline_AllSkipped(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	ctx := &DeleteContext{Branch: "feat/y", SkipStages: allSkipped(len(AllDeleteStages))}
	evs := drain(func(ch chan<- Event) { RunDeletePipeline(ctx, ch) })

	if len(evs) != len(AllDeleteStages)+1 {
		t.Fatalf("got %d events, want %d", len(evs), len(AllDeleteStages)+1)
	}
	if last := evs[len(evs)-1]; last.Type != EventPipelineCompleted {
		t.Errorf("final event = %+v, want PipelineCompleted", last)
	}
}
