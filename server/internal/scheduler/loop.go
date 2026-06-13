package scheduler

import (
	"context"
	"time"
)

func (e *Engine) tickLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Fire once immediately on start to catch any overdue jobs.
	e.tick()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.tick()
		}
	}
}

func (e *Engine) tick() {
	now := e.now().UTC()

	e.mu.Lock()
	var due []Job
	for _, j := range e.jobs {
		if j.Enabled && !j.NextRun.After(now) {
			due = append(due, j)
		}
	}
	e.mu.Unlock()

	for _, j := range due {
		e.fireJob(j, now)
	}
}

func (e *Engine) fireJob(j Job, now time.Time) {
	e.trigger(e.ctx, j)

	nextRun, err := nextCronTime(j.CronExpr, now, e.location)
	if err != nil {
		e.logger.Warn("failed to compute next run for job, disabling",
			"component", "scheduler",
			"job_id", j.JobID,
			"err", err.Error(),
		)
		return
	}

	j.NextRun = nextRun
	j.UpdatedAt = now

	e.mu.Lock()
	if current, ok := e.jobs[j.JobID]; ok {
		current.NextRun = j.NextRun
		current.UpdatedAt = j.UpdatedAt
		j = current
	}
	e.jobs[j.JobID] = j
	e.mu.Unlock()

	// Persist asynchronously; in-memory is authoritative during process lifetime.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = e.repo.UpdateJobSchedule(ctx, j)
	}()
}
