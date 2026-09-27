package dispatch

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

// detachingDeliverer completes every delivery with event.detach and keeps
// the background notifier of each event for the test to end.
type detachingDeliverer struct {
	mu          sync.Mutex
	propagation string
	started     chan chatevent.Event
	background  map[string]*plugins.DetachedEvent
}

func newDetachingDeliverer(propagation string) *detachingDeliverer {
	return &detachingDeliverer{propagation: propagation, started: make(chan chatevent.Event, 8), background: make(map[string]*plugins.DetachedEvent)}
}

func (f *detachingDeliverer) DeliverEvent(_ context.Context, event chatevent.Event) (plugins.Delivery, error) {
	detached := plugins.NewDetachedEvent()
	f.mu.Lock()
	f.background[event.EventID] = detached
	f.mu.Unlock()
	f.started <- event
	return plugins.Delivery{RequestID: event.EventID, Propagation: f.propagation, Result: map[string]any{}, Detached: detached}, nil
}

func (f *detachingDeliverer) ReadyForEvents() bool { return true }

func (f *detachingDeliverer) end(eventID string, err error) {
	f.mu.Lock()
	detached := f.background[eventID]
	f.mu.Unlock()
	detached.Finish(err)
}

func schedulerTestEvent(id string) chatevent.Event {
	return chatevent.Event{EventID: id, SourceProtocol: "scheduler", SourceAdapter: "scheduler.internal", EventType: "scheduler.trigger", Timestamp: time.Now().Unix(), PayloadFields: map[string]any{"task_id": "daily"}}
}

func TestDetachedSchedulerRunIsRecordedWhenTheEventEnds(t *testing.T) {
	t.Parallel()
	d := New(nil, nil, nil, 8)
	defer d.Close()
	rt := newDetachingDeliverer("")
	d.Register("weather", rt, []string{"scheduler.trigger", "message.group"}, nil, 1)
	recorder := &recordingSchedulerRunRecorder{}
	triggeredAt := time.Now().Add(-2 * time.Second)

	result := d.DispatchScheduledEvent(t.Context(), "weather", schedulerTestEvent("run-1"), scheduler.RunContext{JobID: "daily", TaskName: "daily", StartedAt: triggeredAt, Recorder: recorder})
	if completed := waitCompletion(t, result); !completed.Success {
		t.Fatalf("detached delivery completion = %#v", completed)
	}
	waitForStartedEvent(t, rt.started)
	// The released concurrency slot takes the next event while the run is
	// still in the background.
	d.Dispatch(t.Context(), testEvent(), "")
	waitForStartedEvent(t, rt.started)
	if recorder.count() != 0 {
		t.Fatal("the scheduler run was recorded at detach")
	}

	rt.end("run-1", nil)
	waitForCondition(t, func() bool { return recorder.count() == 1 }, "the background end should record the run")
	got := recorder.results()[0]
	if got.JobID != "daily" || got.Outcome != scheduler.RunOutcomeSuccess || got.Duration < 2*time.Second {
		t.Fatalf("background run result = %#v", got)
	}
}

func TestDetachedSchedulerTimeoutIsRecordedWithoutRepeatingTheRuntimeWarning(t *testing.T) {
	t.Parallel()
	logger, stream := newDispatchTestLogger()
	d := New(logger, nil, nil, 8)
	rt := newDetachingDeliverer("")
	d.Register("weather", rt, []string{"scheduler.trigger"}, nil, 1)
	recorder := &recordingSchedulerRunRecorder{}

	result := d.DispatchScheduledEvent(t.Context(), "weather", schedulerTestEvent("run-1"), scheduler.RunContext{JobID: "daily", TaskName: "daily", StartedAt: time.Now(), Recorder: recorder})
	waitCompletion(t, result)
	waitForStartedEvent(t, rt.started)
	failure := &plugins.Error{Code: errorcodes.PluginEventTimeout, Message: "插件后台事件超过期限"}
	failure.MarkFailureReported()
	rt.end("run-1", failure)
	// Close waits for background runs, so the record exists once it returns.
	d.Close()

	results := recorder.results()
	if len(results) != 1 || results[0].Outcome != scheduler.RunOutcomeTimeout || results[0].ErrorCode != errorcodes.PluginEventTimeout {
		t.Fatalf("background timeout record = %#v", results)
	}
	if summary := findDispatchLog(stream, func(summary logging.Summary) bool {
		return summary.Source == "scheduler" && summary.Level == "warn"
	}); summary != nil {
		t.Fatalf("dispatch repeated the runtime warning: %#v", summary)
	}
}

func TestDetachPropagationControlsLowerLayers(t *testing.T) {
	t.Parallel()
	d := New(nil, nil, nil, 8)
	upper := newDetachingDeliverer("stop")
	lower := &fakeDeliverer{}
	d.Register("upper", upper, []string{"message.group"}, nil, 1, MessagePolicy{Priority: 10})
	d.Register("lower", lower, []string{"message.group"}, nil, 1)
	results := d.Dispatch(t.Context(), testEvent(), "")
	if completed := waitCompletion(t, results[0]); !completed.Success || completed.Propagation != "stop" {
		t.Fatalf("detached upper completion = %#v", completed)
	}
	if completed := waitCompletion(t, results[1]); !completed.Skipped {
		t.Fatalf("lower layer ran after a detach stopped propagation: %#v", completed)
	}
	if lower.eventCount() != 0 {
		t.Fatal("lower layer received the message")
	}
	upper.end("test-evt-1", nil)
	d.Close()
}

func TestSchedulerRunDurationIncludesQueueWait(t *testing.T) {
	t.Parallel()
	d := New(nil, nil, nil, 8)
	defer d.Close()
	blocking := &fakeDeliverer{started: make(chan chatevent.Event, 2), blockCh: make(chan struct{})}
	d.Register("weather", blocking, []string{"scheduler.trigger", "message.group"}, nil, 1)
	recorder := &recordingSchedulerRunRecorder{}

	d.Dispatch(t.Context(), testEvent(), "")
	waitForStartedEvent(t, blocking.started)
	d.DispatchScheduledEvent(t.Context(), "weather", schedulerTestEvent("run-1"), scheduler.RunContext{JobID: "daily", TaskName: "daily", StartedAt: time.Now(), Recorder: recorder})
	time.Sleep(200 * time.Millisecond)
	close(blocking.blockCh)
	waitForCondition(t, func() bool { return recorder.count() == 1 }, "the queued run should be recorded")
	if got := recorder.results()[0]; got.Outcome != scheduler.RunOutcomeSuccess || got.Duration < 200*time.Millisecond {
		t.Fatalf("queued run duration = %#v", got)
	}
}
