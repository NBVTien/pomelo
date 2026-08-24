package pipeline

import (
	"github.com/pomelohq/pomelo/internal/provider/shell"
	"os/exec"
	"strings"

	"github.com/pomelohq/pomelo/internal/services"
)

func ExecuteDeleteStage(stage DeleteStage, ctx *DeleteContext) error {
	switch stage {
	case StageStop:
		return deleteStageStop(ctx)
	case StageRelease:
		return deleteStageRelease(ctx)
	case StageCleanup:
		return deleteStageCleanup(ctx)
	case StageRemove:
		return deleteStageRemove(ctx)
	case StageFinalize:
		return deleteStageFinalize(ctx)
	}
	return nil
}

func deleteStageStop(ctx *DeleteContext) error {
	services.StopWorkspaceHolders(ctx.Config, ctx.Branch, ctx.OtherBranches)
	return nil
}

func deleteStageRelease(ctx *DeleteContext) error {
	wsKey := services.PortWsKey(ctx.Branch)
	for name := range ctx.Config.SharedServices {
		services.ReleaseSlot(name, wsKey)
	}
	return nil
}

func deleteStageCleanup(ctx *DeleteContext) error {
	for _, item := range ctx.CleanupItems {
		if len(item.PreDelete) == 0 {
			continue
		}
		if !isDir(item.WtPath) {
			continue
		}
		combined := strings.Join(item.PreDelete, " && ")
		sh := shell.Command(combined)
		cmd := exec.Command(sh[0], sh[1:]...)
		cmd.Dir = item.WtPath
		_ = cmd.Run()
	}
	return nil
}

func deleteStageRemove(ctx *DeleteContext) error {
	for _, item := range ctx.CleanupItems {
		_ = services.RemoveWorktree(item.DirPath, item.WtPath, item.WtBranch)
	}

	if len(ctx.DBsToDrop) > 0 {
		var dbNames []string
		for _, db := range ctx.DBsToDrop {
			dbNames = append(dbNames, db.DBName)
		}
		first := ctx.DBsToDrop[0]
		services.DropSharedDBsBatch(first.Host, first.Port, dbNames, first.User, first.Password)
	}

	services.ReleaseBlock(ctx.ConfigDir, services.PortWsKey(ctx.Branch))
	return nil
}

func deleteStageFinalize(ctx *DeleteContext) error {
	services.DeleteWorkspaceFolder(ctx.ConfigDir, ctx.Branch)
	return nil
}
