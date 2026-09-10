package render

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type failingCloseRunner struct {
	calls   atomic.Int32
	failure error
}

func (*failingCloseRunner) Render(context.Context, Document) ([]byte, error) { return nil, nil }
func (r *failingCloseRunner) Close() error                                   { r.calls.Add(1); return r.failure }

func TestWorkerCloseCancelsQueuedAndActiveRequestsAndPreservesCleanupError(t *testing.T) {
	failure := errors.New("fixture runner close failure")
	runner := &failingCloseRunner{failure: failure}
	queued := make(chan struct{}, 1)
	worker := NewWorker(WorkerConfig{Runner: runner, WorkerCount: 2, QueueMaxLength: 2, OnQueueDepth: func(depth int) {
		if depth == 3 {
			queued <- struct{}{}
		}
	}})
	first, err := worker.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer first()
	second, err := worker.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer second()
	activeCtx, cancel := worker.RenderContext(context.Background())
	defer cancel()
	queueResult := make(chan error, 1)
	go func() {
		release, err := worker.Acquire(context.Background())
		if release != nil {
			release()
		}
		queueResult <- err
	}()
	<-queued
	closeResult := make(chan error, 1)
	go func() { closeResult <- worker.Close() }()
	select {
	case <-activeCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel active rendering")
	}
	select {
	case err := <-queueResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("queued request: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown retained a queued request")
	}
	if release, err := worker.Acquire(context.Background()); release != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("closed worker accepted work: %v", err)
	}
	select {
	case <-closeResult:
		t.Fatal("shutdown did not wait for active owner to release")
	default:
	}
	first()
	second()
	if err := <-closeResult; !errors.Is(err, failure) {
		t.Fatalf("first close lost failure: %v", err)
	}
	if err := worker.Close(); !errors.Is(err, failure) {
		t.Fatalf("repeated close lost failure: %v", err)
	}
	if runner.calls.Load() != 1 {
		t.Fatalf("runner closed %d times", runner.calls.Load())
	}
}

func TestWorkerConcurrentRefreshAndCloseCannotSplitWorkerSlots(t *testing.T) {
	worker := NewWorker(WorkerConfig{Runner: NewChromiumRunner(ChromiumOptions{}), WorkerCount: 4})
	var tasks sync.WaitGroup
	start := make(chan struct{})
	for index := range 24 {
		tasks.Add(1)
		go func() {
			defer tasks.Done()
			<-start
			if index%2 == 0 {
				_ = worker.Close()
			} else {
				worker.RefreshChromiumRunner("fixture-no-process", nil)
			}
		}()
	}
	close(start)
	done := make(chan struct{})
	go func() { tasks.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent close and refresh deadlocked over worker slots")
	}
	if worker.CurrentRunner() != nil || worker.RefreshChromiumRunner("fixture-no-process", nil) {
		t.Fatal("closed worker retained or restarted a browser runner")
	}
}

func TestWorkerQueueTimeoutReleasesReservation(t *testing.T) {
	worker := NewWorker(WorkerConfig{WorkerCount: 1, QueueMaxLength: 1, QueueWaitTimeout: 10 * time.Millisecond, RenderTimeout: 10 * time.Millisecond})
	t.Cleanup(func() { _ = worker.Close() })
	release, err := worker.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	_, err = worker.Acquire(context.Background())
	var renderErr *Error
	if !errors.As(err, &renderErr) || renderErr.Code != "platform.render_timeout" || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queue timeout: %v", err)
	}
	release()
	next, err := worker.Acquire(context.Background())
	if err != nil {
		t.Fatalf("timed-out request retained its reservation: %v", err)
	}
	next()
	renderCtx, cancel := worker.RenderContext(context.Background())
	defer cancel()
	<-renderCtx.Done()
	if !errors.Is(renderCtx.Err(), context.DeadlineExceeded) {
		t.Fatalf("render timeout: %v", renderCtx.Err())
	}
}
