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
	cancels map[string]context.CancelFunc
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

// Submit creates a task in the registry and enqueues it for async execution.
// Returns the task_id on success.
func (e *Executor) Submit(taskType, summary string, fn ExecuteFunc) (string, error) {
	taskID, err := e.registry.Create(taskType, summary)
	if err != nil {
		return "", err
	}

	runCtx, cancel := context.WithTimeout(e.baseCtx, e.timeout)
	e.mu.Lock()
	e.cancels[taskID] = cancel
	e.mu.Unlock()

	select {
	case e.jobs <- executorJob{taskID: taskID, execute: fn, ctx: runCtx}:
		return taskID, nil
	case <-e.baseCtx.Done():
		cancel()
		return "", context.Canceled
	}
}

// Cancel attempts to cancel a running or pending task.
func (e *Executor) Cancel(taskID string) bool {
	snapshot, ok := e.registry.Get(taskID)
	if !ok {
		return false
	}
	if snapshot.Status != StatusPending && snapshot.Status != StatusRunning {
		return false
	}
	e.mu.Lock()
	cancel, ok := e.cancels[taskID]
	e.mu.Unlock()
	if !ok || cancel == nil {
		return false
	}
	cancel()
	if snapshot.Status == StatusPending {
		now := e.now().UTC()
		e.registry.Update(taskID, Update{
			Status:     statusPtr(StatusCancelled),
			Summary:    strPtr("任务已取消"),
			FinishedAt: &now,
		})
		e.dropCancel(taskID)
	}
	return true
}

// Close shuts down the executor and waits for the background goroutine.
func (e *Executor) Close() error {
	if e == nil {
		return nil
	}
	e.baseCancel()
	e.mu.Lock()
	for _, cancel := range e.cancels {
		cancel()
	}
	e.mu.Unlock()
	e.wg.Wait()
	return nil
}

func (e *Executor) run() {
	defer e.wg.Done()
	for {
		select {
		case <-e.baseCtx.Done():
			return
		case job := <-e.jobs:
			e.execute(job)
		}
	}
}

func (e *Executor) execute(job executorJob) {
	defer e.dropCancel(job.taskID)

	snapshot, ok := e.registry.Get(job.taskID)
	if !ok {
		return
	}
	if snapshot.Status == StatusCancelled {
		return
	}

	startedAt := e.now().UTC()
	e.registry.Update(job.taskID, Update{
		Status:    statusPtr(StatusRunning),
		Progress:  intP(0),
		StartedAt: &startedAt,
	})

	reporter := ProgressReporter{registry: e.registry, taskID: job.taskID}
	result, err := job.execute(job.ctx, reporter)

	now := e.now().UTC()
	if err != nil {
		var taskErr *TaskError
		if ok := isTaskError(err, &taskErr); ok {
			e.registry.Update(job.taskID, Update{
				Status:     statusPtr(StatusFailed),
				Summary:    strPtr(taskErr.Message),
				FinishedAt: &now,
				Error: &ErrorSummary{
					Code:    taskErr.Code,
					Message: taskErr.Message,
					Details: taskErr.Details,
				},
			})
		} else {
			code := "platform.internal_error"
			if job.ctx.Err() != nil {
				code = "platform.task_timeout"
			}
			e.registry.Update(job.taskID, Update{
				Status:     statusPtr(StatusFailed),
				Summary:    strPtr(err.Error()),
				FinishedAt: &now,
				Error: &ErrorSummary{
					Code:    code,
					Message: err.Error(),
				},
			})
		}
		return
	}

	if result == nil {
		result = &ResultSummary{Summary: "完成"}
	}
	e.registry.Update(job.taskID, Update{
		Status:     statusPtr(StatusSucceeded),
		Progress:   intP(100),
		Summary:    strPtr(result.Summary),
		FinishedAt: &now,
		Result:     result,
	})
}

func (e *Executor) dropCancel(taskID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.cancels, taskID)
}

func isTaskError(err error, target **TaskError) bool {
	te, ok := err.(*TaskError)
	if ok {
		*target = te
	}
	return ok
}

func statusPtr(s Status) *Status { return &s }
func strPtr(s string) *string    { return &s }
func intP(i int) *int            { return &i }
