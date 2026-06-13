// Package scheduler provides a minimal cron-like job engine with SQLite
// persistence and cross-restart recovery. Jobs are registered by plugin_id
// and fire internal scheduler.trigger events at the configured cadence.
package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

var ErrJobNotFound = errors.New("scheduler job not found")

// Job represents a persisted scheduled job.
type Job struct {
	JobID          string          `json:"job_id"`
	PluginID       string          `json:"plugin_id"`
	LogLabel       string          `json:"log_label,omitempty"`
	CronExpr       string          `json:"cron_expr"`
	Payload        json.RawMessage `json:"payload"`
	Enabled        bool            `json:"enabled"`
	NextRun        time.Time       `json:"next_run"`
	LastRun        *time.Time      `json:"last_run,omitempty"`
	LastDurationMS int64           `json:"last_duration_ms"`
	LastError      *RunError       `json:"last_error,omitempty"`
	RunStats       RunStats        `json:"run_stats"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type RunOutcome string

const (
	RunOutcomeSuccess RunOutcome = "success"
	RunOutcomeFailed  RunOutcome = "failed"
	RunOutcomeTimeout RunOutcome = "timeout"
	RunOutcomeRetry   RunOutcome = "retry"
	RunOutcomeOther   RunOutcome = "other"
)

type RunError struct {
	Code    string    `json:"code"`
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

type RunStats struct {
	Success int64 `json:"success"`
	Failed  int64 `json:"failed"`
	Timeout int64 `json:"timeout"`
	Retry   int64 `json:"retry"`
	Other   int64 `json:"other"`
}

func (s RunStats) Total() int64 {
	return s.Success + s.Failed + s.Timeout + s.Retry + s.Other
}

type RunResult struct {
	JobID      string
	Outcome    RunOutcome
	Duration   time.Duration
	ErrorCode  string
	ErrorText  string
	OccurredAt time.Time
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
