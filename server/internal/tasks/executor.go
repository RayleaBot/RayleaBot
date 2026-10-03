package tasks

import (
	"context"
	"errors"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
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
	baseCancel context.CancelCauseFunc
	wg         sync.WaitGroup
	jobs       chan executorJob
	admission  *QueueAdmission

	mu      sync.Mutex
	closed  bool
	cancels map[string]context.CancelFunc
}

type executorJob struct {
	taskID  string
	execute ExecuteFunc
	ctx     context.Context
}

var errExecutorShutdown = errors.New("task executor shutdown")

// NewExecutor creates a new generic task executor with the given default
// timeout per job. The executor starts a single background goroutine that
// processes submitted jobs sequentially.
func NewExecutor(registry *Registry, timeout time.Duration) *Executor {
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	baseCtx, baseCancel := context.WithCancelCause(context.Background())
	e := &Executor{
		registry:   registry,
		timeout:    timeout,
		now:        time.Now,
		baseCtx:    baseCtx,
		baseCancel: baseCancel,
		jobs:       make(chan executorJob, 32),
		admission:  NewQueueAdmission(32),
		cancels:    map[string]context.CancelFunc{},
	}
	e.wg.Add(1)
	go e.run()
	return e
}

func (e *Executor) Submit(taskType, summary string, fn ExecuteFunc) (string, error) {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return "", context.Canceled
	}
	if !e.admission.TryAcquire() {
		e.mu.Unlock()
		return "", ErrQueueFull
	}

	taskID, err := e.registry.Create(taskType, summary)
	if err != nil {
		e.admission.Release()
		e.mu.Unlock()
		return "", err
	}

	runCtx, cancel := context.WithTimeout(e.baseCtx, e.timeout)
	e.cancels[taskID] = cancel

	e.jobs <- executorJob{taskID: taskID, execute: fn, ctx: runCtx}
	e.mu.Unlock()
	return taskID, nil
}

func (e *Executor) Get(taskID string) (Snapshot, bool) {
	return e.registry.Get(taskID)
}

func (e *Executor) List() []Snapshot {
	if e.registry == nil {
		return nil
	}
	return e.registry.List()
}

func (e *Executor) Close() error {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		e.wg.Wait()
		return nil
	}
	e.closed = true
	e.baseCancel(errExecutorShutdown)
	e.mu.Unlock()
	e.wg.Wait()
	return nil
}

func (e *Executor) run() {
	defer e.wg.Done()
	for {
		select {
		case <-e.baseCtx.Done():
			// Settle every accepted queued task before flushing the registry.
			for {
				select {
				case job := <-e.jobs:
					e.admission.Release()
					e.execute(job)
				default:
					return
				}
			}
		case job := <-e.jobs:
			e.admission.Release()
			e.execute(job)
		}
	}
}

func (e *Executor) execute(job executorJob) {
	defer e.dropCancel(job.taskID)

	e.mu.Lock()
	if _, ok := e.registry.Get(job.taskID); !ok {
		e.mu.Unlock()
		return
	}
	if err := job.ctx.Err(); err != nil {
		e.mu.Unlock()
		e.finishError(job, err)
		return
	}

	startedAt := e.now().UTC()
	e.registry.Update(job.taskID, Update{
		Status:    statusPtr(StatusRunning),
		Progress:  intP(0),
		StartedAt: &startedAt,
	})
	e.mu.Unlock()

	reporter := ProgressReporter{registry: e.registry, taskID: job.taskID}
	result, err := job.execute(job.ctx, reporter)

	now := e.now().UTC()
	if err != nil {
		e.finishError(job, err)
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

func (e *Executor) finishError(job executorJob, err error) {
	now := e.now().UTC()
	var taskErr *TaskError
	if !isTaskError(err, &taskErr) && onlyCancellation(err) {
		status, summary := StatusCancelled, "任务已取消"
		if errors.Is(context.Cause(job.ctx), errExecutorShutdown) {
			status, summary = StatusInterrupted, "任务因服务关闭而中断"
		}
		e.registry.Update(job.taskID, Update{Status: &status, Summary: &summary, FinishedAt: &now})
		return
	}
	if taskErr != nil {
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
		code := errorcodes.PlatformInternalError
		if errors.Is(err, context.DeadlineExceeded) {
			code = errorcodes.PlatformTaskTimeout
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
}

// Joined cleanup failures remain failures even when the initiating operation
// was cancelled. A wrapper around cancellation alone retains its identity.
func onlyCancellation(err error) bool {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !onlyCancellation(child) {
				return false
			}
		}
		return true
	}
	if inner := errors.Unwrap(err); inner != nil {
		return onlyCancellation(inner)
	}
	return errors.Is(err, context.Canceled)
}

func (e *Executor) dropCancel(taskID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if cancel := e.cancels[taskID]; cancel != nil {
		cancel()
	}
	delete(e.cancels, taskID)
}

func isTaskError(err error, target **TaskError) bool {
	return errors.As(err, target)
}

func statusPtr(s Status) *Status { return &s }
func strPtr(s string) *string    { return &s }
func intP(i int) *int            { return &i }
