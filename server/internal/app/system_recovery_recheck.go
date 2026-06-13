package app

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func (s *systemService) submitRecoveryRecheckTask() (string, *systemHTTPError) {
	if s == nil || s.taskExecutor == nil {
		return "", internalSystemHTTPError()
	}

	summary, err := recovery.LoadSummary(s.state.repoRoot)
	if err != nil {
		return "", internalSystemHTTPError()
	}
	if summary == nil || (!summary.RequiresPostStartChecks && summary.Phase != "post_startup") {
		return "", missingSystemResourceHTTPError(recoverySummaryDetails(s.state.repoRoot))
	}

	taskID, err := s.taskExecutor.Submit("recovery.recheck", "重新检查恢复摘要", func(ctx context.Context, progress tasks.ProgressReporter) (*tasks.ResultSummary, error) {
		progress.Update(25, "读取恢复摘要")
		reconciled, err := s.reconcileRecoverySummary()
		if err != nil {
			return nil, err
		}
		if reconciled == nil {
			return nil, &tasks.TaskError{
				Code:    codeResourceMissing,
				Message: "恢复摘要不存在或当前不可重新检查",
				Details: recoverySummaryDetails(s.state.repoRoot),
			}
		}
		progress.Update(90, "写入恢复摘要")
		return &tasks.ResultSummary{
			Summary: "恢复摘要已重新检查",
			Details: map[string]any{
				"recovery_summary": reconciled,
			},
		}, nil
	})
	if err != nil {
		return "", internalSystemHTTPError()
	}
	return taskID, nil
}
