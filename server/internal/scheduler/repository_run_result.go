package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

func (r *SQLiteRepository) RecordJobRunResult(ctx context.Context, result RunResult) error {
	if result.OccurredAt.IsZero() {
		result.OccurredAt = time.Now().UTC()
	}
	if result.Duration < 0 {
		result.Duration = 0
	}
	outcome := normalizeRunOutcome(result.Outcome)
	lastRun := result.OccurredAt.UTC().Format(time.RFC3339Nano)

	if outcome == RunOutcomeSuccess {
		if err := r.writeQ.RecordJobRunSuccess(ctx, sqlcgen.RecordJobRunSuccessParams{
			LastRun:        sql.NullString{String: lastRun, Valid: true},
			LastDurationMs: result.Duration.Milliseconds(),
			UpdatedAt:      lastRun,
			JobID:          result.JobID,
		}); err != nil {
			return fmt.Errorf("record scheduler job run result %s: %w", result.JobID, err)
		}
		return nil
	}

	failed, timeoutCount, retry, other := runOutcomeCounters(outcome)
	if err := r.writeQ.RecordJobRunFailure(ctx, sqlcgen.RecordJobRunFailureParams{
		LastRun:          sql.NullString{String: lastRun, Valid: true},
		LastDurationMs:   result.Duration.Milliseconds(),
		LastErrorCode:    DisplayLabel(result.ErrorCode, string(outcome)),
		LastErrorMessage: DisplayLabel(result.ErrorText, string(outcome)),
		LastErrorAt:      sql.NullString{String: lastRun, Valid: true},
		FailureCount:     failed,
		TimeoutCount:     timeoutCount,
		RetryCount:       retry,
		OtherCount:       other,
		UpdatedAt:        lastRun,
		JobID:            result.JobID,
	}); err != nil {
		return fmt.Errorf("record scheduler job run result %s: %w", result.JobID, err)
	}
	return nil
}

func runOutcomeCounters(outcome RunOutcome) (failed int64, timeoutCount int64, retry int64, other int64) {
	switch outcome {
	case RunOutcomeFailed:
		failed = 1
	case RunOutcomeTimeout:
		timeoutCount = 1
	case RunOutcomeRetry:
		retry = 1
	case RunOutcomeOther:
		other = 1
	}
	return failed, timeoutCount, retry, other
}
