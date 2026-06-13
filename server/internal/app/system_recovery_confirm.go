package app

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func (s *systemService) validateRecoveryConfirmRequest(reviewIDs []string, note string) *systemHTTPError {
	if s == nil {
		return internalSystemHTTPError()
	}
	summary, err := recovery.LoadSummary(s.state.repoRoot)
	if err != nil {
		return internalSystemHTTPError()
	}
	if summary == nil || summary.Phase != "post_startup" {
		return missingSystemResourceHTTPError(recoverySummaryDetails(s.state.repoRoot))
	}
	if _, _, err := recovery.ConfirmSkippedPlugins(*summary, reviewIDs, "validation", note, "validation"); err != nil {
		var unknownErr *recovery.UnknownReviewIDsError
		if errors.As(err, &unknownErr) {
			return invalidSystemHTTPError(map[string]any{"review_ids": unknownErr.ReviewIDs})
		}
		return invalidSystemHTTPError(nil)
	}
	return nil
}

func (s *systemService) submitRecoveryConfirmTask(reviewIDs []string, note, operatorID string) (string, *systemHTTPError) {
	if s == nil || s.taskExecutor == nil {
		return "", internalSystemHTTPError()
	}
	taskIDCh := make(chan string, 1)
	taskID, err := s.taskExecutor.Submit("recovery.confirm", "确认恢复处理结果", func(ctx context.Context, progress tasks.ProgressReporter) (*tasks.ResultSummary, error) {
		progress.Update(20, "读取恢复摘要")
		currentSummary, err := recovery.LoadSummary(s.state.repoRoot)
		if err != nil {
			return nil, err
		}
		if currentSummary == nil || currentSummary.Phase != "post_startup" {
			return nil, &tasks.TaskError{
				Code:    codeResourceMissing,
				Message: "恢复摘要不存在或当前不可确认",
				Details: recoverySummaryDetails(s.state.repoRoot),
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
		if err := recovery.SaveSummary(s.state.repoRoot, confirmedSummary); err != nil {
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
		return "", internalSystemHTTPError()
	}
	taskIDCh <- taskID
	return taskID, nil
}
