package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pomelohq/pomelo/internal/paths"
)

type EventType int

const (
	EventStageStarted EventType = iota
	EventStageCompleted
	EventStageSkipped
	EventStageProgress
	EventPipelineCompleted
	EventPipelineFailed
)

type Event struct {
	Type   EventType
	Branch string
	Index  int
	Name   string
	Total  int
	Error  string
	Detail string
}

type StageStatus string

const (
	StatusPending   StageStatus = "Pending"
	StatusCompleted StageStatus = "Completed"
	StatusSkipped   StageStatus = "Skipped"
	StatusFailed    StageStatus = "Failed"
)

type StageEntry struct {
	Name   string      `json:"name"`
	Status StageStatus `json:"status"`
	Error  string      `json:"error,omitempty"`
}

type PipelineOp string

const (
	OpCreateWorkspace PipelineOp = "CreateWorkspace"
	OpDeleteWorkspace PipelineOp = "DeleteWorkspace"
)

type PipelineState struct {
	Operation   PipelineOp   `json:"operation"`
	Branch      string       `json:"branch"`
	Workspace   string       `json:"workspace"`
	Stages      []StageEntry `json:"stages"`
	FailedStage int          `json:"failed_stage"`
}

func activeMarkerDir() string {
	return paths.StatePath("active")
}

func MarkPipelineActive(branch string, stage, total int, stageName string) {
	dir := activeMarkerDir()
	_ = os.MkdirAll(dir, 0o755)
	content := fmt.Sprintf("%d/%d %s", stage, total, stageName)
	_ = os.WriteFile(filepath.Join(dir, strings.ReplaceAll(branch, "/", "-")), []byte(content), 0o644)
}

func MarkPipelineDone(branch string) {
	path := filepath.Join(activeMarkerDir(), strings.ReplaceAll(branch, "/", "-"))
	_ = os.Remove(path)
}

func pipelineStatePath(branch string) string {
	return paths.StatePath(fmt.Sprintf("pipeline-%s.json", strings.ReplaceAll(branch, "/", "-")))
}

func SavePipelineState(branch, workspace string, op PipelineOp, stageLabels []string, failedStage int, errMsg string) {
	var stages []StageEntry
	for i, name := range stageLabels {
		status := StatusPending
		e := ""
		if i < failedStage {
			status = StatusCompleted
		} else if i == failedStage {
			status = StatusFailed
			e = errMsg
		}
		stages = append(stages, StageEntry{Name: name, Status: status, Error: e})
	}

	state := PipelineState{
		Operation:   op,
		Branch:      branch,
		Workspace:   workspace,
		Stages:      stages,
		FailedStage: failedStage,
	}

	path := pipelineStatePath(branch)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := json.MarshalIndent(state, "", "  ")
	_ = os.WriteFile(path, data, 0o644)
}

func ClearPipelineState(branch string) {
	_ = os.Remove(pipelineStatePath(branch))
}
