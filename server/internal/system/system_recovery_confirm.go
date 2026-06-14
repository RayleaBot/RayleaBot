package system

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	systemmodel "github.com/RayleaBot/RayleaBot/server/internal/system/model"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func (s *Service) ValidateRecoveryConfirmRequest(reviewIDs []string, note string) *systemmodel.Error {
	if s == nil {
		return systemmodel.InternalError()
	}
	summary, err := recovery.LoadSummary(s.repoRootPath())
	if err != nil {
		return systemmodel.InternalError()
	}
	if summary == nil || summary.Phase != "post_startup" {
		return systemmodel.ResourceMissingError(systemmodel.RecoverySummaryDetails(s.repoRootPath()))
	}
	if _, _, err := recovery.ConfirmSkippedPlugins(*summary, reviewIDs, "validation", note, "validation"); err != nil {
		var unknownErr *recovery.UnknownReviewIDsError
		if errors.As(err, &unknownErr) {
			return systemmodel.InvalidRequestError(map[string]any{"review_ids": unknownErr.ReviewIDs})
		}
		return systemmodel.InvalidRequestError(nil)
	}
	return nil
}

func (s *Service) SubmitRecoveryConfirmTask(reviewIDs []string, note, operatorID string) (string, *systemmodel.Error) {
	if s == nil || s.taskExecutor == nil {
		return "", systemmodel.InternalError()
	}
	taskIDCh := make(chan string, 1)
	taskID, err := s.taskExecutor.Submit("recovery.confirm", "确认恢复处理结果", func(ctx context.Context, progress tasks.ProgressReporter) (*tasks.ResultSummary, error) {
		progress.Update(20, "读取恢复摘要")
		currentSummary, err := recovery.LoadSummary(s.repoRootPath())
		if err != nil {
			return nil, err
		}
		if currentSummary == nil || currentSummary.Phase != "post_startup" {
			return nil, &tasks.TaskError{
				Code:    codeResourceMissing,
				Message: "恢复摘要不存在或当前不可确认",
				Details: systemmodel.RecoverySummaryDetails(s.repoRootPath()),
			}
		}
		progress.Update(55, "确认恢复项")
		taskID := <-taskIDCh
		confirmedSummary, confirmedReviewIDs, err := recovery.ConfirmSkippedPlugins(*currentSummary, reviewIDs, operatorID, note, taskID)
		if err != nil {
			var unknownErr *recovery.UnknownReviewIDsError
			if errors.As(err, &unknownErr) {
				return nil, &tasks.TaskError{
					Code:    codeInvalidRequest,
					Message: "请求参数不合法",
					Details: map[string]any{
						"review_ids": unknownErr.ReviewIDs,
					},
				}
			}
			return nil, err
		}
		progress.Update(85, "写入恢复摘要")
		if err := recovery.SaveSummary(s.repoRootPath(), confirmedSummary); err != nil {
			return nil, err
		}
		s.applyRecoverySummary(&confirmedSummary)

		summaryText := "所选恢复项已确认"
		if len(confirmedReviewIDs) == 0 {
			summaryText = "所选恢复项已经确认"
		}
		return &tasks.ResultSummary{
			Summary: summaryText,
			Details: map[string]any{
				"recovery_summary":     confirmedSummary,
				"confirmed_review_ids": confirmedReviewIDs,
				"operator_id":          operatorID,
				"note":                 note,
			},
		}, nil
	})
	if err != nil {
		return "", systemmodel.InternalError()
	}
	taskIDCh <- taskID
	return taskID, nil
}
