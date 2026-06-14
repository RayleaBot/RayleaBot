package engine

import (
	"context"
	"errors"
	"sync"
	"time"

	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
)

type Config struct {
	Runner           renderbrowser.Runner
	WorkerCount      int
	QueueMaxLength   int
	QueueWaitTimeout time.Duration
	RenderTimeout    time.Duration
	OnQueueDepth     func(depth int)
}

type Limits struct {
	QueueMaxLength   int
	QueueWaitTimeout time.Duration
	RenderTimeout    time.Duration
}

type Worker struct {
	mu               sync.RWMutex
	runner           renderbrowser.Runner
	slots            chan struct{}
	workerCount      int
	queueMaxLength   int
	queueWaitTimeout time.Duration
	renderTimeout    time.Duration
	activeRequests   int
	onQueueDepth     func(depth int)
}

func New(config Config) *Worker {
	workerCount := config.WorkerCount
	if workerCount <= 0 {
		workerCount = 1
	}
	return &Worker{
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
	if w == nil {
		return nil, &rendertemplates.Error{Code: "platform.resource_missing", Message: "render worker is not available"}
	}

	if err := w.reserveSlot(); err != nil {
		return nil, err
	}
	releaseRequest := true
	release := func() {
		if !releaseRequest {
			return
		}
		releaseRequest = false
		w.releaseSlot()
	}

	queueCtx := ctx
	cancel := func() {}
	if timeout := w.QueueWaitTimeout(); timeout > 0 {
		queueCtx, cancel = context.WithTimeout(ctx, timeout)
	}

	select {
	case w.slots <- struct{}{}:
		cancel()
		return func() {
			<-w.slots
			release()
		}, nil
	case <-queueCtx.Done():
		cancel()
		release()
		return nil, &rendertemplates.Error{
			Code:    "platform.render_timeout",
			Message: "render queue wait timed out",
			Err:     queueCtx.Err(),
		}
	}
}

func (w *Worker) RenderContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if w == nil {
		return ctx, func() {}
	}
	if timeout := w.RenderTimeout(); timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return ctx, func() {}
}

func (w *Worker) CurrentRunner() renderbrowser.Runner {
	if w == nil {
		return nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.runner
}

func (w *Worker) UpdateLimits(limits Limits) {
	if w == nil {
		return
	}
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
	if w == nil {
		return 0
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.queueWaitTimeout
}

func (w *Worker) RenderTimeout() time.Duration {
	if w == nil {
		return 0
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.renderTimeout
}

func (w *Worker) Close() error {
	if w == nil {
		return nil
	}
	releaseWorkers := w.acquireAllWorkerSlots()
	defer releaseWorkers()

	w.mu.Lock()
	runner := w.runner
	w.runner = nil
	w.mu.Unlock()
	return closeRunner(runner)
}

func (w *Worker) RefreshChromiumRunner(browserPath string, browserArgs []string) bool {
	if w == nil {
		return false
	}

	w.mu.RLock()
	oldRunner := w.runner
	replaceDefaultRunner := renderbrowser.IsChromiumRunner(oldRunner)
	w.mu.RUnlock()
	if !replaceDefaultRunner {
		return false
	}

	releaseWorkers := w.acquireAllWorkerSlots()
	defer releaseWorkers()

	w.mu.Lock()
	if w.runner != oldRunner {
		w.mu.Unlock()
		return false
	}
	w.runner = renderbrowser.NewChromiumRunner(renderbrowser.ChromiumOptions{
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

	limit := w.workerCount + w.queueMaxLength
	if limit <= 0 {
		limit = w.workerCount
	}
	if w.activeRequests >= limit {
		w.publishQueueDepthLocked()
		return &rendertemplates.Error{
			Code:    "platform.render_queue_full",
			Message: "render queue is full",
		}
	}
	w.activeRequests++
	w.publishQueueDepthLocked()
	return nil
}

func (w *Worker) releaseSlot() {
	w.mu.Lock()
	defer w.mu.Unlock()
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
	if w == nil || w.slots == nil {
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
		return &rendertemplates.Error{
			Code:    "platform.render_timeout",
			Message: "render execution timed out",
			Err:     err,
		}
	}
	return err
}

type closeableRunner interface {
	Close() error
}

func closeRunner(runner renderbrowser.Runner) error {
	closeable, ok := runner.(closeableRunner)
	if !ok || closeable == nil {
		return nil
	}
	return closeable.Close()
}
