package tasks

import (
	"context"
	"sync"
	"time"
)

// ExecuteFunc is the function signature for task execution logic.
// It receives a context (which may be cancelled) and a progress reporter.
// It should return a ResultSummary on success or an error.
type ExecuteFunc func(ctx context.Context, progress ProgressReporter) (*ResultSummary, error)

// ProgressReporter allows task implementations to report progress updates.
type ProgressReporter struct {
	registry *Registry
	taskID   string
}

// Update reports a progress update for the running task.
func (p ProgressReporter) Update(percent int, summary string) {
	p.registry.Update(p.taskID, Update{
		Progress: &percent,
		Summary:  &summary,
	})
}

// TaskError represents a structured task failure with an error code.
type TaskError struct {
	Code    string
	Message string
	Details map[string]any
}

func (e *TaskError) Error() string { return e.Message }

// MetricsObserver lets callers route task execution outcomes into the
// platform Prometheus registry without forcing this package to depend on
// client_golang. Implementations must be safe for concurrent use.
type MetricsObserver interface {
	ObserveTaskExecution(taskType, outcome string, duration time.Duration)
}

// Executor provides a reusable async task execution loop. It accepts jobs
// via Submit, runs them on a single background goroutine, and drives task
// status through the Registry.
type Executor struct {
	registry *Registry
	timeout  time.Duration
	now      func() time.Time

	baseCtx    context.Context
	baseCancel context.CancelFunc
	wg         sync.WaitGroup
	jobs       chan executorJob

	mu      sync.Mutex
	closed  bool
	cancels map[string]context.CancelFunc

	metricsMu sync.RWMutex
	metrics   MetricsObserver
}

type executorJob struct {
	taskID  string
	execute ExecuteFunc
	ctx     context.Context
}

// NewExecutor creates a new generic task executor with the given default
// timeout per job. The executor starts a single background goroutine that
// processes submitted jobs sequentially.
func NewExecutor(registry *Registry, timeout time.Duration) *Executor {
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	baseCtx, baseCancel := context.WithCancel(context.Background())
	e := &Executor{
		registry:   registry,
		timeout:    timeout,
		now:        time.Now,
		baseCtx:    baseCtx,
		baseCancel: baseCancel,
		jobs:       make(chan executorJob, 32),
		cancels:    map[string]context.CancelFunc{},
	}
	e.wg.Add(1)
	go e.run()
	return e
}
