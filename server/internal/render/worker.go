package render

import (
	"context"
	"errors"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"sync"
	"time"
)

type WorkerConfig struct {
	Runner           Runner
	WorkerCount      int
	QueueMaxLength   int
	QueueWaitTimeout time.Duration
	RenderTimeout    time.Duration
	OnQueueDepth     func(depth int)
}

type WorkerLimits struct {
	QueueMaxLength   int
	QueueWaitTimeout time.Duration
	RenderTimeout    time.Duration
}

type Worker struct {
	mu               sync.RWMutex
	lifecycleMu      sync.Mutex
	ctx              context.Context
	cancel           context.CancelFunc
	closed           bool
	closeComplete    bool
	closeErr         error
	requests         sync.WaitGroup
	runner           Runner
	slots            chan struct{}
	workerCount      int
	queueMaxLength   int
	queueWaitTimeout time.Duration
	renderTimeout    time.Duration
	activeRequests   int
	onQueueDepth     func(depth int)
}

func NewWorker(config WorkerConfig) *Worker {
	workerCount := config.WorkerCount
	if workerCount <= 0 {
		workerCount = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		ctx: ctx, cancel: cancel,
		runner:           config.Runner,
		slots:            make(chan struct{}, workerCount),
		workerCount:      workerCount,
		queueMaxLength:   config.QueueMaxLength,
		queueWaitTimeout: config.QueueWaitTimeout,
		renderTimeout:    config.RenderTimeout,
		onQueueDepth:     config.OnQueueDepth,
	}
}

func (w *Worker) Acquire(ctx context.Context) (func(), error) {
	if err := w.reserveSlot(); err != nil {
		return nil, err
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(w.releaseSlot) }

	queueCtx := ctx
	cancel := func() {}
	if timeout := w.QueueWaitTimeout(); timeout > 0 {
		queueCtx, cancel = context.WithTimeout(ctx, timeout)
	}

	select {
	case w.slots <- struct{}{}:
		cancel()
		if err := w.ctx.Err(); err != nil {
			<-w.slots
			release()
			return nil, err
		}
		var slotOnce sync.Once
		return func() {
			slotOnce.Do(func() { <-w.slots; release() })
		}, nil
	case <-w.ctx.Done():
		cancel()
		release()
		return nil, w.ctx.Err()
	case <-queueCtx.Done():
		cancel()
		release()
		return nil, &Error{
			Code:    errorcodes.PlatformRenderTimeout,
			Message: "render queue wait timed out",
			Err:     queueCtx.Err(),
		}
	}
}

func (w *Worker) RenderContext(ctx context.Context) (context.Context, context.CancelFunc) {
	var cancel context.CancelFunc
	if timeout := w.RenderTimeout(); timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	stop := context.AfterFunc(w.ctx, cancel)
	return ctx, func() { stop(); cancel() }
}

func (w *Worker) CurrentRunner() Runner {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.runner
}

func (w *Worker) UpdateLimits(limits WorkerLimits) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if limits.QueueMaxLength > 0 {
		w.queueMaxLength = limits.QueueMaxLength
	}
	if limits.QueueWaitTimeout > 0 {
		w.queueWaitTimeout = limits.QueueWaitTimeout
	}
	if limits.RenderTimeout > 0 {
		w.renderTimeout = limits.RenderTimeout
	}
}

func (w *Worker) QueueWaitTimeout() time.Duration {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.queueWaitTimeout
}

func (w *Worker) RenderTimeout() time.Duration {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.renderTimeout
}

func (w *Worker) Close() error {
	w.mu.Lock()
	w.closed = true
	w.cancel()
	w.mu.Unlock()
	w.lifecycleMu.Lock()
	defer w.lifecycleMu.Unlock()
	if w.closeComplete {
		return w.closeErr
	}
	w.requests.Wait()
	w.mu.Lock()
	runner := w.runner
	w.runner = nil
	w.mu.Unlock()
	w.closeErr = closeRunner(runner)
	w.closeComplete = true
	return w.closeErr
}

func (w *Worker) RefreshChromiumRunner(browserPath string, browserArgs []string) bool {
	w.lifecycleMu.Lock()
	defer w.lifecycleMu.Unlock()
	w.mu.RLock()
	oldRunner := w.runner
	replaceDefaultRunner := !w.closed && IsChromiumRunner(oldRunner)
	w.mu.RUnlock()
	if !replaceDefaultRunner {
		return false
	}

	releaseWorkers := w.acquireAllWorkerSlots()
	defer releaseWorkers()

	w.mu.Lock()
	if w.closed || w.runner != oldRunner {
		w.mu.Unlock()
		return false
	}
	w.runner = NewChromiumRunner(ChromiumOptions{
		BrowserPath: browserPath,
		BrowserArgs: append([]string(nil), browserArgs...),
	})
	w.mu.Unlock()
	_ = closeRunner(oldRunner)
	return true
}

func (w *Worker) reserveSlot() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return &Error{Code: errorcodes.PlatformResourceMissing, Message: "render worker is closed", Err: context.Canceled}
	}

	limit := w.workerCount + w.queueMaxLength
	if limit <= 0 {
		limit = w.workerCount
	}
	if w.activeRequests >= limit {
		w.publishQueueDepthLocked()
		return &Error{
			Code:    errorcodes.PlatformRenderQueueFull,
			Message: "render queue is full",
		}
	}
	w.activeRequests++
	w.requests.Add(1)
	w.publishQueueDepthLocked()
	return nil
}

func (w *Worker) releaseSlot() {
	w.mu.Lock()
	defer w.mu.Unlock()
	defer w.requests.Done()
	if w.activeRequests > 0 {
		w.activeRequests--
	}
	w.publishQueueDepthLocked()
}

func (w *Worker) publishQueueDepthLocked() {
	if w.onQueueDepth == nil {
		return
	}
	depth := w.activeRequests
	w.onQueueDepth(depth)
}

func (w *Worker) acquireAllWorkerSlots() func() {
	if w.slots == nil {
		return func() {}
	}
	count := cap(w.slots)
	for i := 0; i < count; i++ {
		w.slots <- struct{}{}
	}
	return func() {
		for i := 0; i < count; i++ {
			<-w.slots
		}
	}
}

func WrapRenderError(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return &Error{
			Code:    errorcodes.PlatformRenderTimeout,
			Message: "render execution timed out",
			Err:     err,
		}
	}
	return err
}

type closeableRunner interface {
	Close() error
}

func closeRunner(runner Runner) error {
	closeable, ok := runner.(closeableRunner)
	if !ok || closeable == nil {
		return nil
	}
	return closeable.Close()
}
