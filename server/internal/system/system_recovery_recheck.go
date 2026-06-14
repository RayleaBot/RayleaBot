package system

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	systemmodel "github.com/RayleaBot/RayleaBot/server/internal/system/model"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func (s *Service) SubmitRecoveryRecheckTask() (string, *systemmodel.Error) {
	if s == nil || s.taskExecutor == nil {
		return "", systemmodel.InternalError()
	}

	summary, err := recovery.LoadSummary(s.repoRootPath())
	if err != nil {
		return "", systemmodel.InternalError()
	}
	if summary == nil || (!summary.RequiresPostStartChecks && summary.Phase != "post_startup") {
		return "", systemmodel.ResourceMissingError(systemmodel.RecoverySummaryDetails(s.repoRootPath()))
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
				Details: systemmodel.RecoverySummaryDetails(s.repoRootPath()),
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
		return "", systemmodel.InternalError()
	}
	return taskID, nil
}
