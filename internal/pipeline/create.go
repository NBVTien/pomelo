package pipeline

import (
	"github.com/pomelohq/pomelo/internal/services"
)

type CreateState struct {
	WsFolder   string
	BranchSafe string
	WtDirs     []services.DirMapping
}

func NewCreateState(ctx *CreateContext) *CreateState {
	return &CreateState{
		BranchSafe: services.BranchSafe(ctx.Branch),
	}
}

func ExecuteCreateStage(stage CreateStage, ctx *CreateContext, state *CreateState) error {
	switch stage {
	case StageValidate:
		return stageValidate(ctx)
	case StageProvision:
		return stageProvision(ctx, state)
	case StageInfra:
		return stageInfra(ctx, state)
	case StageSource:
		return stageSourceParallel(ctx, state)
	case StageConfigure:
		return stageConfigureParallel(ctx, state)
	case StageSetup:
		return stageSetupParallel(ctx, state)
	case StageSeed:
		return stageSeedParallel(ctx, state)
	}
	return nil
}
