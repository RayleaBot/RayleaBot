// Package scheduler provides a minimal cron-like job engine with SQLite
// persistence and cross-restart recovery. Jobs are registered by plugin_id
// and fire internal scheduler.trigger events at the configured cadence.
package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Job represents a persisted scheduled job.
type Job struct {
	JobID     string          `json:"job_id"`
	PluginID  string          `json:"plugin_id"`
	CronExpr  string          `json:"cron_expr"`
	Payload   json.RawMessage `json:"payload"`
	Enabled   bool            `json:"enabled"`
	NextRun   time.Time       `json:"next_run"`
	LastRun   *time.Time      `json:"last_run,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// TriggerFunc is called when a job fires. The engine passes the job metadata
// so the caller can route the trigger to the correct plugin runtime.
type TriggerFunc func(ctx context.Context, job Job)

// Engine is the scheduler engine. It maintains an in-memory set of jobs,
// persists them to a Repository, and runs a tick loop that fires due jobs.
type Engine struct {
	repo     Repository
	logger   *slog.Logger
	trigger  TriggerFunc
	location *time.Location
	now      func() time.Time

	mu   sync.Mutex
	jobs map[string]Job

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Options configures the scheduler engine.
type Options struct {
	Repository Repository
	Logger     *slog.Logger
	Trigger    TriggerFunc
	Timezone   string
}

// New creates a scheduler engine. Call Start to begin the tick loop.
func New(opts Options) (*Engine, error) {
	if opts.Repository == nil {
		return nil, fmt.Errorf("scheduler repository is required")
	}
	if opts.Logger == nil {
		return nil, fmt.Errorf("scheduler logger is required")
	}

	loc := time.UTC
	if opts.Timezone != "" {
		parsed, err := time.LoadLocation(opts.Timezone)
		if err != nil {
			return nil, fmt.Errorf("load scheduler timezone %q: %w", opts.Timezone, err)
		}
		loc = parsed
	}

	trigger := opts.Trigger
	if trigger == nil {
		trigger = func(context.Context, Job) {}
	}

	return &Engine{
		repo:     opts.Repository,
		logger:   opts.Logger,
		trigger:  trigger,
		location: loc,
		now:      time.Now,
		jobs:     make(map[string]Job),
	}, nil
}

// Hydrate loads persisted jobs into the in-memory map. Should be called once
// before Start.
func (e *Engine) Hydrate(ctx context.Context) error {
	jobs, err := e.repo.LoadJobs(ctx)
	if err != nil {
		return fmt.Errorf("hydrate scheduler: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	for _, j := range jobs {
		e.jobs[j.JobID] = j
	}

	e.logger.Info("scheduler hydrated", "component", "scheduler", "job_count", len(jobs))
	return nil
}

// Start begins the background tick loop. It should be called after Hydrate.
func (e *Engine) Start(ctx context.Context) {
	e.ctx, e.cancel = context.WithCancel(ctx)
	e.wg.Add(1)
	go e.tickLoop()
}

// Stop cancels the tick loop and waits for it to finish.
func (e *Engine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	e.wg.Wait()
}

// Register creates a new scheduled job and persists it.
func (e *Engine) Register(ctx context.Context, pluginID, cronExpr string, payload json.RawMessage) (Job, error) {
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

	e.logger.Info("scheduler job registered",
		"component", "scheduler",
		"job_id", job.JobID,
		"plugin_id", pluginID,
		"cron_expr", cronExpr,
		"next_run", nextRun.Format(time.RFC3339),
	)

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

	j.LastRun = &now
	j.NextRun = nextRun
	j.UpdatedAt = now

	e.mu.Lock()
	e.jobs[j.JobID] = j
	e.mu.Unlock()

	// Persist asynchronously — in-memory is authoritative during process lifetime.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = e.repo.SaveJob(ctx, j)
	}()
}

func generateJobID() (string, error) {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate job id: %w", err)
	}
	return "sched_" + hex.EncodeToString(buf[:]), nil
}
