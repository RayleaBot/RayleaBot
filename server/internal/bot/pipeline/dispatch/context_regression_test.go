package dispatch

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

type contextDeliverer struct {
	contexts chan context.Context
	release  chan struct{}
}

type stoppingDeliverer struct {
	started chan struct{}
	stopped atomic.Bool
}

func (rt *stoppingDeliverer) DeliverEvent(ctx context.Context, _ chatevent.Event) (plugins.Delivery, error) {
	close(rt.started)
	<-ctx.Done()
	rt.stopped.Store(true)
	return plugins.Delivery{}, ctx.Err()
}

func TestShutdownCountsQueuedSchedulerRunsAsCanceled(t *testing.T) {
	d := New(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, 4)
	rt := &stoppingDeliverer{started: make(chan struct{})}
	recorder := &checkingRunRecorder{results: make(chan scheduler.RunResult, 2), err: make(chan error, 2)}
	d.Register("fixture", rt, nil, nil, 1)
	event := testEvent()
	event.EventType = "scheduler.trigger"
	run := scheduler.RunContext{JobID: "fixture-job", TaskName: "fixture-job", StartedAt: time.Now(), Recorder: recorder}
	d.DispatchScheduledEvent(t.Context(), "fixture", event, run)
	<-rt.started
	d.DispatchScheduledEvent(t.Context(), "fixture", event, run)
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := d.DrainAll(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled drain: %v", err)
	}
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
	results chan scheduler.RunResult
	err     chan error
}

func (r *checkingRunRecorder) RecordRunResult(ctx context.Context, result scheduler.RunResult) error {
	r.err <- ctx.Err()
	r.results <- result
	return ctx.Err()
}

type cancelingDeliverer struct{ started chan struct{} }

func (rt *cancelingDeliverer) DeliverEvent(ctx context.Context, _ chatevent.Event) (plugins.Delivery, error) {
	close(rt.started)
	<-ctx.Done()
	return plugins.Delivery{}, ctx.Err()
}

func TestShutdownPersistsCanceledSchedulerResultExactlyOnce(t *testing.T) {
	d := New(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, 4)
	rt := &cancelingDeliverer{started: make(chan struct{})}
	recorder := &checkingRunRecorder{results: make(chan scheduler.RunResult, 2), err: make(chan error, 2)}
	d.Register("fixture", rt, nil, nil, 1)
	event := testEvent()
	event.EventType = "scheduler.trigger"
	run := scheduler.RunContext{JobID: "fixture-job", TaskName: "fixture-job", StartedAt: time.Now(), Recorder: recorder}
	d.DispatchScheduledEvent(t.Context(), "fixture", event, run)
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
	var failure *plugins.Error
	recorder := &recordingSchedulerRunRecorder{}
	d.Register("fixture", &fakeDeliverer{err: failure}, nil, nil, 1)
	event := testEvent()
	event.EventType = "scheduler.trigger"
	run := scheduler.RunContext{JobID: "fixture-job", TaskName: "fixture-job", StartedAt: time.Now(), Recorder: recorder}
	d.DispatchScheduledEvent(t.Context(), "fixture", event, run)
	d.Close()
	if recorder.count() != 1 {
		t.Fatal("worker did not finish the failed run")
	}
}

func (rt *contextDeliverer) DeliverEvent(ctx context.Context, _ chatevent.Event) (plugins.Delivery, error) {
	rt.contexts <- ctx
	<-rt.release
	return plugins.Delivery{}, ctx.Err()
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
	// Completion releases this delivery's context even when the submitter's
	// earlier cancellation did not end the admitted event.
	waitForContextDone(t, accepted, time.Second)
}

func TestSuccessiveDeliveriesHaveIndependentContextsAndTracing(t *testing.T) {
	d := New(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, 4)
	t.Cleanup(d.Close)
	rt := &contextDeliverer{contexts: make(chan context.Context, 1), release: make(chan struct{})}
	t.Cleanup(func() { close(rt.release) })
	d.Register("fixture", rt, nil, nil, 1)
	type traceKey struct{}
	for sequence := range 24 {
		ctx := context.WithValue(t.Context(), traceKey{}, sequence)
		result := d.DispatchToPlugin(ctx, "fixture", testEvent())
		if result.Outcome != OutcomeDelivered {
			t.Fatal(result)
		}
		accepted := <-rt.contexts
		if accepted.Err() != nil || accepted.Value(traceKey{}) != sequence {
			t.Fatal("delivery retained an earlier context or trace")
		}
		rt.release <- struct{}{}
		if !waitCompletion(t, result).Success {
			t.Fatal("delivery failed")
		}
		waitForContextDone(t, accepted, time.Second)
	}
}

func waitForContextDone(t *testing.T, ctx context.Context, timeout time.Duration) {
	t.Helper()
	select {
	case <-ctx.Done():
	case <-time.After(timeout):
		t.Fatal("dispatcher close did not cancel owned context")
	}
}

// ReadyForEvents reports whether this target can accept a plugin event.
func (rt *stoppingDeliverer) ReadyForEvents() bool {
	return !rt.stopped.Load()
}

// ReadyForEvents reports whether this target can accept a plugin event.
func (rt *cancelingDeliverer) ReadyForEvents() bool {
	return true
}

// ReadyForEvents reports whether this target can accept a plugin event.
func (rt *contextDeliverer) ReadyForEvents() bool {
	return true
}
