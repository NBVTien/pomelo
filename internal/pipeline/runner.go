package pipeline

func RunCreatePipeline(ctx *CreateContext, ch chan<- Event) {
	stages := AllCreateStages
	total := len(stages)
	branch := ctx.Branch
	state := NewCreateState(ctx)
	ctx.Progress = func(msg string) { ch <- Event{Type: EventStageProgress, Branch: branch, Detail: msg} }

	MarkPipelineActive(branch, 0, total, stages[0].Label())

	for i, stage := range stages {
		if ctx.SkipStages != nil && ctx.SkipStages[i] {
			ch <- Event{Type: EventStageSkipped, Branch: branch, Index: i}
			continue
		}

		MarkPipelineActive(branch, i, total, stage.Label())
		ch <- Event{Type: EventStageStarted, Branch: branch, Index: i, Name: stage.Label(), Total: total}

		if err := ExecuteCreateStage(stage, ctx, state); err != nil {
			labels := make([]string, len(stages))
			for j, s := range stages {
				labels[j] = s.Label()
			}
			SavePipelineState(ctx.Branch, ctx.WorkspaceName, OpCreateWorkspace, labels, i, err.Error())
			MarkPipelineDone(branch)
			ch <- Event{Type: EventPipelineFailed, Branch: branch, Index: i, Error: err.Error()}
			return
		}

		ch <- Event{Type: EventStageCompleted}
	}

	ClearPipelineState(ctx.Branch)
	MarkPipelineDone(branch)
	ch <- Event{Type: EventPipelineCompleted, Branch: branch}
}

func RunDeletePipeline(ctx *DeleteContext, ch chan<- Event) {
	stages := AllDeleteStages
	total := len(stages)
	branch := ctx.Branch

	MarkPipelineActive(branch, 0, total, stages[0].Label())

	for i, stage := range stages {
		if ctx.SkipStages != nil && ctx.SkipStages[i] {
			ch <- Event{Type: EventStageSkipped, Branch: branch, Index: i}
			continue
		}

		MarkPipelineActive(branch, i, total, stage.Label())
		ch <- Event{Type: EventStageStarted, Branch: branch, Index: i, Name: stage.Label(), Total: total}

		if err := ExecuteDeleteStage(stage, ctx); err != nil {
			labels := make([]string, len(stages))
			for j, s := range stages {
				labels[j] = s.Label()
			}
			SavePipelineState(ctx.Branch, "", OpDeleteWorkspace, labels, i, err.Error())
			MarkPipelineDone(branch)
			ch <- Event{Type: EventPipelineFailed, Branch: branch, Index: i, Error: err.Error()}
			return
		}

		ch <- Event{Type: EventStageCompleted}
	}

	ClearPipelineState(ctx.Branch)
	MarkPipelineDone(branch)
	ch <- Event{Type: EventPipelineCompleted, Branch: branch}
}
