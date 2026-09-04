package dispatch

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

type contextDeliverer struct {
	contexts chan context.Context
	release  chan struct{}
}

type stoppingDeliverer struct {
	started chan struct{}
	stopped atomic.Bool
}

func (rt *stoppingDeliverer) Snapshot() pluginruntime.Snapshot {
	if rt.stopped.Load() {
		return pluginruntime.Snapshot{State: pluginruntime.StateStopped}
	}
	return pluginruntime.Snapshot{State: pluginruntime.StateRunning}
}

func (rt *stoppingDeliverer) DeliverEvent(ctx context.Context, _ pluginruntime.Event) (pluginruntime.Delivery, error) {
	close(rt.started)
	<-ctx.Done()
	rt.stopped.Store(true)
	return pluginruntime.Delivery{}, ctx.Err()
}

func TestShutdownCountsQueuedSchedulerRunsAsCanceled(t *testing.T) {
	d := New(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, 4)
	rt := &stoppingDeliverer{started: make(chan struct{})}
	recorder := &checkingRunRecorder{results: make(chan pluginruntime.SchedulerRunResult, 2), err: make(chan error, 2)}
	d.Register("fixture", rt, nil, nil, 1)
	event := testEvent()
	event.EventType = "scheduler.trigger"
	event.SchedulerLog = &pluginruntime.SchedulerLogContext{JobID: "fixture-job", TaskName: "fixture-job", StartedAt: time.Now(), Recorder: recorder}
	d.DispatchToPlugin(t.Context(), "fixture", event)
	<-rt.started
	d.DispatchToPlugin(t.Context(), "fixture", event)
	d.CancelPending()
	d.Close()
	if len(recorder.results) != 2 {
		t.Fatalf("recorded runs = %d, want 2", len(recorder.results))
	}
	for range 2 {
		result := <-recorder.results
		if result.Outcome != "other" || result.ErrorCode != "plugin.event_canceled" {
			t.Fatalf("shutdown run misclassified: %#v", result)
		}
		if err := <-recorder.err; err != nil {
			t.Fatalf("result persistence canceled: %v", err)
		}
	}
}

type checkingRunRecorder struct {
	results chan pluginruntime.SchedulerRunResult
	err     chan error
}

func (r *checkingRunRecorder) RecordSchedulerRunResult(ctx context.Context, result pluginruntime.SchedulerRunResult) error {
	r.err <- ctx.Err()
	r.results <- result
	return ctx.Err()
}

type cancelingDeliverer struct{ started chan struct{} }

func (rt *cancelingDeliverer) Snapshot() pluginruntime.Snapshot {
	return pluginruntime.Snapshot{State: pluginruntime.StateRunning}
}
func (rt *cancelingDeliverer) DeliverEvent(ctx context.Context, _ pluginruntime.Event) (pluginruntime.Delivery, error) {
	close(rt.started)
	<-ctx.Done()
	return pluginruntime.Delivery{}, ctx.Err()
}

func TestShutdownPersistsCanceledSchedulerResultExactlyOnce(t *testing.T) {
	d := New(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, 4)
	rt := &cancelingDeliverer{started: make(chan struct{})}
	recorder := &checkingRunRecorder{results: make(chan pluginruntime.SchedulerRunResult, 2), err: make(chan error, 2)}
	d.Register("fixture", rt, nil, nil, 1)
	event := testEvent()
	event.EventType = "scheduler.trigger"
	event.SchedulerLog = &pluginruntime.SchedulerLogContext{JobID: "fixture-job", TaskName: "fixture-job", StartedAt: time.Now(), Recorder: recorder}
	d.DispatchToPlugin(t.Context(), "fixture", event)
	<-rt.started
	d.Close()
	if err := <-recorder.err; err != nil {
		t.Fatalf("canceled storage context: %v", err)
	}
	if len(recorder.results) != 1 {
		t.Fatalf("saved results = %d", len(recorder.results))
	}
	result := <-recorder.results
	if result.Outcome != "other" || result.ErrorCode != "plugin.event_canceled" {
		t.Fatalf("cancellation misclassified: %#v", result)
	}
}

func TestTypedNilFailureDoesNotCrashWorker(t *testing.T) {
	logger, _ := newDispatchTestLogger()
	d := New(logger, nil, nil, 1)
	var failure *pluginruntime.Error
	recorder := &recordingSchedulerRunRecorder{}
	d.Register("fixture", &fakeDeliverer{err: failure}, nil, nil, 1)
	event := testEvent()
	event.EventType = "scheduler.trigger"
	event.SchedulerLog = &pluginruntime.SchedulerLogContext{JobID: "fixture-job", TaskName: "fixture-job", StartedAt: time.Now(), Recorder: recorder}
	d.DispatchToPlugin(t.Context(), "fixture", event)
	d.Close()
	if recorder.count() != 1 {
		t.Fatal("worker did not finish the failed run")
	}
}

func (rt *contextDeliverer) Snapshot() pluginruntime.Snapshot {
	return pluginruntime.Snapshot{State: pluginruntime.StateRunning}
}
func (rt *contextDeliverer) DeliverEvent(ctx context.Context, _ pluginruntime.Event) (pluginruntime.Delivery, error) {
	rt.contexts <- ctx
	<-rt.release
	return pluginruntime.Delivery{}, ctx.Err()
}

func TestAcceptedAsyncEventOutlivesCallerCancellation(t *testing.T) {
	d := New(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, 4)
	rt := &contextDeliverer{contexts: make(chan context.Context, 1), release: make(chan struct{})}
	d.Register("fixture", rt, nil, nil, 1)
	ctx, cancel := context.WithCancel(t.Context())
	if result := d.DispatchToPlugin(ctx, "fixture", testEvent()); result.Outcome != OutcomeDelivered {
		t.Fatal(result)
	}
	accepted := <-rt.contexts
	cancel()
	if accepted.Err() != nil {
		t.Fatal("accepted event inherited short-lived cancellation")
	}
	if result := d.DispatchToPlugin(ctx, "fixture", testEvent()); result.ErrorCode != "plugin.event_canceled" {
		t.Fatal("canceled admission accepted", result)
	}
	close(rt.release)
	d.Close()
	if accepted.Err() == nil {
		t.Fatal("dispatcher close did not cancel owned context")
	}
}
