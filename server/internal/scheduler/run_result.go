package scheduler

import (
	"context"
	"fmt"

	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

// Trigger fires a registered job immediately without advancing the scheduled
// next run time.
func (e *Engine) Trigger(ctx context.Context, jobID string) (Job, error) {
	e.mu.Lock()
	job, ok := e.jobs[jobID]
	e.mu.Unlock()
	if !ok {
		return Job{}, ErrJobNotFound
	}
	if !job.Enabled {
		return Job{}, ErrJobNotFound
	}
	if ctx == nil {
		ctx = context.Background()
	}
	e.trigger(ctx, job)
	return job, nil
}

func (e *Engine) RecordRunResult(ctx context.Context, result RunResult) error {
	if e == nil {
		return ErrJobNotFound
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if result.OccurredAt.IsZero() {
		result.OccurredAt = e.now().UTC()
	}
	result.OccurredAt = result.OccurredAt.UTC()
	if result.Duration < 0 {
		result.Duration = 0
	}
	result.Outcome = normalizeRunOutcome(result.Outcome)

	e.mu.Lock()
	job, ok := e.jobs[result.JobID]
	if !ok {
		e.mu.Unlock()
		return ErrJobNotFound
	}
	lastRun := result.OccurredAt
	job.LastRun = &lastRun
	job.LastDurationMS = result.Duration.Milliseconds()
	applyRunOutcome(&job.RunStats, result.Outcome)
	if result.Outcome != RunOutcomeSuccess {
		job.LastError = &RunError{
			Code:    DisplayLabel(result.ErrorCode, string(result.Outcome)),
			Message: DisplayLabel(result.ErrorText, string(result.Outcome)),
			At:      result.OccurredAt,
		}
	}
	job.UpdatedAt = result.OccurredAt
	e.jobs[job.JobID] = job
	e.mu.Unlock()

	if err := e.repo.RecordJobRunResult(ctx, result); err != nil {
		return fmt.Errorf("record scheduler run result %s: %w", result.JobID, err)
	}
	return nil
}

func (e *Engine) RecordSchedulerRunResult(ctx context.Context, result runtime.SchedulerRunResult) error {
	return e.RecordRunResult(ctx, RunResult{
		JobID:      result.JobID,
		Outcome:    RunOutcome(result.Outcome),
		Duration:   result.Duration,
		ErrorCode:  result.ErrorCode,
		ErrorText:  result.ErrorText,
		OccurredAt: result.OccurredAt,
	})
}

func normalizeRunOutcome(outcome RunOutcome) RunOutcome {
	switch outcome {
	case RunOutcomeSuccess, RunOutcomeFailed, RunOutcomeTimeout, RunOutcomeRetry, RunOutcomeOther:
		return outcome
	default:
		return RunOutcomeOther
	}
}

func applyRunOutcome(stats *RunStats, outcome RunOutcome) {
	switch normalizeRunOutcome(outcome) {
	case RunOutcomeSuccess:
		stats.Success++
	case RunOutcomeFailed:
		stats.Failed++
	case RunOutcomeTimeout:
		stats.Timeout++
	case RunOutcomeRetry:
		stats.Retry++
	case RunOutcomeOther:
		stats.Other++
	}
}

func cloneRunError(err *RunError) *RunError {
	if err == nil {
		return nil
	}
	cloned := *err
	return &cloned
}
