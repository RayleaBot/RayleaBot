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
	e.markRunning(1)
	defer e.markRunning(-1)

	e.trigger(e.ctx, j)

	nextRun, err := nextCronTime(j.CronExpr, now, e.location)
	if err != nil {
		e.logger.Warn("定时任务 "+j.JobID+" 的下次运行时间计算失败，已停止继续调度",
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

func (e *Engine) markRunning(delta int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.running += delta
	if e.running < 0 {
		e.running = 0
	}
}
