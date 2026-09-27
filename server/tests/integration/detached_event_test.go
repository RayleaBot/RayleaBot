package integration

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

const detachedPluginID = "detached-fixture"

// TestDetachedEventPluginProcess is the plugin process of the detached event
// test: the test binary re-executes itself and runs the Go SDK over stdio.
func TestDetachedEventPluginProcess(t *testing.T) {
	if os.Getenv("RAYLEABOT_DETACHED_PLUGIN") != "1" {
		return
	}
	if err := rayleabot.Run(context.Background(), rayleabot.Options{}, rayleabot.HandlerFunc(detachedFixtureHandler)); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
	os.Exit(0)
}

// detachedFixtureHandler writes long flows as sequential code: it detaches,
// keeps acting under the same event and then ends it.
func detachedFixtureHandler(ctx context.Context, event *rayleabot.EventContext) error {
	switch event.Event.EventType {
	case "scheduler.trigger":
		if _, err := event.Detach(ctx, nil); err != nil {
			return err
		}
		if !strings.HasPrefix(event.Event.TaskID(), "finish") {
			<-ctx.Done()
			return ctx.Err()
		}
		// The gate stands for minutes of work that outlive the event deadline.
		if _, err := event.Actions().KVGet(ctx, "gate"); err != nil {
			return err
		}
		return event.Result(nil)
	case "message.group":
		if _, err := event.DetachWithPropagation(ctx, nil, rayleabot.PropagationStop); err != nil {
			return err
		}
		return event.Reply(event.Event.EventID, true, rayleabot.Text("background reply"))
	case "management.action":
		if _, err := event.Detach(ctx, map[string]any{"started": true}); err != nil {
			return err
		}
		if _, err := event.Actions().KVGet(ctx, "gate"); err != nil {
			return err
		}
		return event.Result(nil)
	}
	return nil
}

type gateCall struct {
	eventID string
	release chan struct{}
}

type runResults struct {
	mu      sync.Mutex
	results []scheduler.RunResult
	changed chan struct{}
}

func (r *runResults) RecordRunResult(_ context.Context, result scheduler.RunResult) error {
	r.mu.Lock()
	r.results = append(r.results, result)
	r.mu.Unlock()
	r.changed <- struct{}{}
	return nil
}

func (r *runResults) await(t *testing.T, jobID string) scheduler.RunResult {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		r.mu.Lock()
		for _, result := range r.results {
			if result.JobID == jobID {
				r.mu.Unlock()
				return result
			}
		}
		r.mu.Unlock()
		select {
		case <-r.changed:
		case <-deadline:
			t.Fatalf("scheduler run %s was not recorded", jobID)
		}
	}
}

func (r *runResults) has(jobID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, result := range r.results {
		if result.JobID == jobID {
			return true
		}
	}
	return false
}

func TestDetachedEventsAcrossSDKProcess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var timeoutSeconds atomic.Int64
	timeoutSeconds.Store(60)
	runtimeConfig := func() config.RuntimeConfig {
		return config.RuntimeConfig{PluginDetachedEventTimeoutSeconds: int(timeoutSeconds.Load()), MaxDetachedEventsPerPlugin: 8}
	}
	gates := make(chan gateCall, 4)
	replies := make(chan plugins.Action, 4)
	execute := func(ctx context.Context, _ string, _ string, action plugins.Action, parent chatevent.Event) (map[string]any, error) {
		switch action.Kind {
		case "storage.kv":
			call := gateCall{eventID: parent.EventID, release: make(chan struct{})}
			gates <- call
			select {
			case <-call.release:
				return map[string]any{"key": action.StorageKey, "exists": false}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		case "message.send":
			replies <- action
			return map[string]any{"message_id": "reply-1"}, nil
		}
		return nil, errors.New("unexpected fixture action " + action.Kind)
	}
	start := func() *pluginruntime.Manager {
		t.Helper()
		manager := pluginruntime.NewManager(logger, pluginruntime.Options{ExecuteLocalAction: execute, RuntimeConfig: runtimeConfig})
		spec := pluginruntime.Spec{
			PluginID: detachedPluginID, PluginName: "后台事件夹具", Command: os.Args[0],
			Args: []string{"-test.run=^TestDetachedEventPluginProcess$"}, Env: []string{"RAYLEABOT_DETACHED_PLUGIN=1"},
			WorkDir: t.TempDir(), InitTimeout: 10 * time.Second, EventTimeout: 3 * time.Second, ShutdownGrace: 2 * time.Second,
			EffectiveConcurrency: 1, IPCMessageMaxBytes: 8 * 1024 * 1024, ValidateFrames: true,
		}
		if err := manager.Start(t.Context(), spec, pluginruntime.InitPayload{Timezone: "UTC", Config: map[string]any{}, CommandPrefixes: []string{"/"}}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = manager.Stop(ctx)
		})
		return manager
	}
	d := dispatch.New(logger, nil, nil, 16)
	t.Cleanup(d.Close)
	manager := start()
	events := []string{"scheduler.trigger", "message.group"}
	d.Register(detachedPluginID, manager, events, nil, 1)
	recorder := &runResults{changed: make(chan struct{}, 16)}
	trigger := func(task string) dispatch.DeliveryResult {
		t.Helper()
		result := d.DispatchScheduledEvent(t.Context(), detachedPluginID, chatevent.Event{
			EventID: "scheduler-" + task, SourceProtocol: "scheduler", SourceAdapter: "scheduler.internal", EventType: "scheduler.trigger",
			Timestamp: time.Now().Unix(), PayloadFields: map[string]any{"task_id": task},
		}, scheduler.RunContext{JobID: task, TaskName: task, StartedAt: time.Now(), Recorder: recorder})
		completed, err := waitDispatchCompletion(t, result)
		if err != nil || !completed.Success {
			t.Fatalf("scheduler %s delivery = %#v, %v", task, completed, err)
		}
		return result
	}
	awaitGate := func(eventID string) gateCall {
		t.Helper()
		select {
		case call := <-gates:
			if call.eventID != eventID {
				t.Fatalf("gate called by %s, want %s", call.eventID, eventID)
			}
			return call
		case <-time.After(5 * time.Second):
			t.Fatalf("%s did not act in the background", eventID)
			return gateCall{}
		}
	}

	t.Run("scheduler run ends in the background after the lane is released", func(t *testing.T) {
		trigger("finish")
		gate := awaitGate("scheduler-finish")
		// With concurrency 1, a message still runs while the run is detached.
		results := d.Dispatch(t.Context(), chatevent.Event{
			EventID: "message-1", SourceProtocol: "onebot11", SourceAdapter: "onebot", EventType: "message.group", Timestamp: time.Now().Unix(),
			Actor: &chatevent.Actor{ID: "10001"}, Target: &chatevent.Target{Type: "group", ID: "20001"}, Message: &chatevent.Message{PlainText: "更新"},
		}, "")
		completed, err := waitDispatchCompletion(t, results[0])
		if err != nil || !completed.Success || completed.Propagation != "stop" {
			t.Fatalf("detached message completion = %#v, %v", completed, err)
		}
		select {
		case reply := <-replies:
			if reply.ReplyToEventID != "message-1" || reply.TargetID != "20001" {
				t.Fatalf("background reply = %#v", reply)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("background reply was not sent as an action")
		}
		if recorder.has("finish") {
			t.Fatal("the run was recorded before the background event ended")
		}
		close(gate.release)
		if result := recorder.await(t, "finish"); result.Outcome != scheduler.RunOutcomeSuccess {
			t.Fatalf("background run = %#v", result)
		}
	})

	t.Run("management caller receives the detach result", func(t *testing.T) {
		delivery, err := manager.DeliverEvent(t.Context(), chatevent.Event{
			EventID: "management-1", SourceProtocol: "management", SourceAdapter: "management.ui", EventType: "management.action",
			Timestamp: time.Now().Unix(), PayloadFields: map[string]any{"action": "refresh", "payload": map[string]any{}},
		})
		if err != nil || delivery.Result["started"] != true || delivery.Detached == nil {
			t.Fatalf("management delivery = %#v, %v", delivery, err)
		}
		close(awaitGate("management-1").release)
		select {
		case <-delivery.Detached.Done():
			if err := delivery.Detached.Err(); err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("management flow did not end")
		}
	})

	t.Run("background deadline ends the run as a timeout", func(t *testing.T) {
		timeoutSeconds.Store(1)
		defer timeoutSeconds.Store(60)
		trigger("expire")
		result := recorder.await(t, "expire")
		if result.Outcome != scheduler.RunOutcomeTimeout || result.ErrorCode != errorcodes.PluginEventTimeout {
			t.Fatalf("expired run = %#v", result)
		}
	})

	t.Run("reload cancels background events of the retired process", func(t *testing.T) {
		trigger("reload")
		replacement := start()
		drain, err := d.SwapPlugin(detachedPluginID, replacement, events, nil, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := drain.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
		stopCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		if err := manager.Stop(stopCtx); err != nil {
			t.Fatal(err)
		}
		if result := recorder.await(t, "reload"); result.Outcome != scheduler.RunOutcomeOther || result.ErrorCode != errorcodes.PluginEventCanceled {
			t.Fatalf("reloaded run = %#v", result)
		}
		trigger("finish-after-reload")
		close(awaitGate("scheduler-finish-after-reload").release)
		if result := recorder.await(t, "finish-after-reload"); result.Outcome != scheduler.RunOutcomeSuccess {
			t.Fatalf("replacement run = %#v", result)
		}
	})
}

func waitDispatchCompletion(t *testing.T, result dispatch.DeliveryResult) (dispatch.CompletionResult, error) {
	t.Helper()
	if result.Completion == nil {
		t.Fatalf("delivery was not accepted: %#v", result)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	return result.Completion.Wait(ctx)
}
