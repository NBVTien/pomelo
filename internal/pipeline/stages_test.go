package pipeline

import "testing"

func TestCreateStageSequence(t *testing.T) {
	want := []struct {
		stage CreateStage
		label string
	}{
		{StageValidate, "Validating config and hosts"},
		{StageProvision, "Provisioning workspace"},
		{StageInfra, "Starting shared services and databases"},
		{StageSource, "Creating git worktrees (parallel)"},
		{StageConfigure, "Configuring repos (parallel)"},
		{StageSetup, "Running setup commands (parallel)"},
		{StageSeed, "Seeding databases (parallel)"},
	}
	if len(AllCreateStages) != len(want) {
		t.Fatalf("AllCreateStages has %d stages, want %d", len(AllCreateStages), len(want))
	}
	for i, w := range want {
		if AllCreateStages[i] != w.stage {
			t.Errorf("AllCreateStages[%d] = %d, want %d", i, AllCreateStages[i], w.stage)
		}
		if int(w.stage) != i {
			t.Errorf("stage %q has iota %d, expected position %d", w.label, w.stage, i)
		}
		if got := w.stage.Label(); got != w.label {
			t.Errorf("stage %d Label() = %q, want %q", i, got, w.label)
		}
	}
}

func TestDeleteStageSequence(t *testing.T) {
	want := []struct {
		stage DeleteStage
		label string
	}{
		{StageStop, "Stopping services"},
		{StageRelease, "Releasing IP and slots"},
		{StageCleanup, "Running pre-delete commands"},
		{StageRemove, "Removing worktrees and databases"},
		{StageFinalize, "Cleaning up folders"},
	}
	if len(AllDeleteStages) != len(want) {
		t.Fatalf("AllDeleteStages has %d stages, want %d", len(AllDeleteStages), len(want))
	}
	for i, w := range want {
		if AllDeleteStages[i] != w.stage {
			t.Errorf("AllDeleteStages[%d] = %d, want %d", i, AllDeleteStages[i], w.stage)
		}
		if int(w.stage) != i {
			t.Errorf("stage %q has iota %d, expected position %d", w.label, w.stage, i)
		}
		if got := w.stage.Label(); got != w.label {
			t.Errorf("stage %d Label() = %q, want %q", i, got, w.label)
		}
	}
}

func TestUnknownStageLabel(t *testing.T) {
	if got := CreateStage(99).Label(); got != "Unknown" {
		t.Errorf("CreateStage(99).Label() = %q, want %q", got, "Unknown")
	}
	if got := DeleteStage(99).Label(); got != "Unknown" {
		t.Errorf("DeleteStage(99).Label() = %q, want %q", got, "Unknown")
	}
}
