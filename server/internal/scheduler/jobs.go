package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Register creates a new scheduled job and persists it.
func (e *Engine) Register(ctx context.Context, pluginID, cronExpr string, payload json.RawMessage) (Job, error) {
	return e.RegisterWithLabel(ctx, pluginID, "", cronExpr, payload)
}

func (e *Engine) RegisterWithLabel(ctx context.Context, pluginID, logLabel, cronExpr string, payload json.RawMessage) (Job, error) {
	now := e.now().UTC()

	nextRun, err := nextCronTime(cronExpr, now, e.location)
	if err != nil {
		return Job{}, fmt.Errorf("parse cron expression %q: %w", cronExpr, err)
	}

	jobID, err := generateJobID()
	if err != nil {
		return Job{}, err
	}

	if payload == nil {
		payload = json.RawMessage("{}")
	}

	job := Job{
		JobID:     jobID,
		PluginID:  pluginID,
		LogLabel:  logLabel,
		CronExpr:  cronExpr,
		Payload:   payload,
		Enabled:   true,
		NextRun:   nextRun,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := e.repo.SaveJob(ctx, job); err != nil {
		return Job{}, fmt.Errorf("persist scheduled job: %w", err)
	}

	e.mu.Lock()
	e.jobs[job.JobID] = job
	e.mu.Unlock()

	return job, nil
}

// UpsertTask creates or updates a plugin-owned scheduled job keyed by task_id.
// For plugin-created jobs the task_id is the persisted job_id, making the
// operation idempotent across repeated scheduler.create calls.
func (e *Engine) UpsertTask(ctx context.Context, pluginID, taskID, cronExpr string, payload json.RawMessage) (Job, error) {
	return e.UpsertTaskWithLabel(ctx, pluginID, taskID, "", cronExpr, payload)
}

func (e *Engine) UpsertTaskWithLabel(ctx context.Context, pluginID, taskID, logLabel, cronExpr string, payload json.RawMessage) (Job, error) {
	now := e.now().UTC()

	nextRun, err := nextCronTime(cronExpr, now, e.location)
	if err != nil {
		return Job{}, fmt.Errorf("parse cron expression %q: %w", cronExpr, err)
	}
	if payload == nil {
		payload = json.RawMessage("{}")
	}

	job := Job{
		JobID:     taskID,
		PluginID:  pluginID,
		LogLabel:  logLabel,
		CronExpr:  cronExpr,
		Payload:   payload,
		Enabled:   true,
		NextRun:   nextRun,
		CreatedAt: now,
		UpdatedAt: now,
	}

	e.mu.Lock()
	if existing, ok := e.jobs[taskID]; ok {
		job.CreatedAt = existing.CreatedAt
		if existing.LastRun != nil {
			lastRun := *existing.LastRun
			job.LastRun = &lastRun
		}
		job.LastDurationMS = existing.LastDurationMS
		job.LastError = cloneRunError(existing.LastError)
		job.RunStats = existing.RunStats
	}
	e.mu.Unlock()

	if err := e.repo.SaveJob(ctx, job); err != nil {
		return Job{}, fmt.Errorf("upsert scheduled task %s: %w", taskID, err)
	}

	e.mu.Lock()
	e.jobs[job.JobID] = job
	e.mu.Unlock()

	return job, nil
}

// Unregister removes a scheduled job.
func (e *Engine) Unregister(ctx context.Context, jobID string) error {
	e.mu.Lock()
	delete(e.jobs, jobID)
	e.mu.Unlock()

	if err := e.repo.DeleteJob(ctx, jobID); err != nil {
		return fmt.Errorf("delete scheduled job %s: %w", jobID, err)
	}
	return nil
}

// UnregisterByPlugin removes all jobs for a given plugin.
func (e *Engine) UnregisterByPlugin(ctx context.Context, pluginID string) error {
	e.mu.Lock()
	var toDelete []string
	for id, j := range e.jobs {
		if j.PluginID == pluginID {
			toDelete = append(toDelete, id)
		}
	}
	for _, id := range toDelete {
		delete(e.jobs, id)
	}
	e.mu.Unlock()

	if err := e.repo.DeleteJobsByPlugin(ctx, pluginID); err != nil {
		return fmt.Errorf("delete jobs for plugin %s: %w", pluginID, err)
	}
	return nil
}

// Jobs returns a snapshot of all registered jobs.
func (e *Engine) Jobs() []Job {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make([]Job, 0, len(e.jobs))
	for _, j := range e.jobs {
		result = append(result, j)
	}
	return result
}

func (e *Engine) RunningCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

func generateJobID() (string, error) {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate job id: %w", err)
	}
	return "sched_" + hex.EncodeToString(buf[:]), nil
}
