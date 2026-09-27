package runtime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginwire"
)

// pipeRuntime drives a Manager over in-memory pipes; the test plays the plugin
// process. Every host frame is checked against the protocol schema.
type pipeRuntime struct {
	t       *testing.T
	manager *Manager
	handle  *Handle
	frames  chan map[string]any
	output  *io.PipeWriter
	logs    *lockedBuffer
	actions chan executedAction
}

type executedAction struct {
	kind   string
	parent chatevent.Event
	ctx    context.Context
}

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

func (b *lockedBuffer) records(t *testing.T) []map[string]any {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	var records []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(b.buffer.Bytes()))
	for decoder.More() {
		var record map[string]any
		if err := decoder.Decode(&record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	return records
}

func startPipeRuntime(t *testing.T, runtimeConfig func() config.RuntimeConfig, execute LocalActionExecutor) *pipeRuntime {
	t.Helper()
	logs := &lockedBuffer{}
	actions := make(chan executedAction, 16)
	if execute == nil {
		execute = func(ctx context.Context, _ string, _ string, action plugins.Action, parent chatevent.Event) (map[string]any, error) {
			actions <- executedAction{kind: action.Kind, parent: parent, ctx: ctx}
			return map[string]any{}, nil
		}
	}
	manager := NewManager(slog.New(slog.NewJSONHandler(logs, nil)), Options{
		ExecuteLocalAction: execute,
		RuntimeConfig:      runtimeConfig,
	})
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	handle := NewHandle(nil, stdinWriter, bufio.NewReader(stdoutReader), ProcessSpec{PluginID: "fixture", EventTimeout: 5 * time.Second, ShutdownGrace: time.Second, EffectiveConcurrency: 4})
	manager.proc = handle
	manager.snap = Snapshot{PluginID: "fixture", State: StateRunning}
	frames := make(chan map[string]any, 64)
	go func() {
		defer close(frames)
		scanner := bufio.NewScanner(stdinReader)
		scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
		for scanner.Scan() {
			if err := pluginwire.Validate(scanner.Bytes(), 0); err != nil {
				t.Errorf("host frame violates the protocol schema: %v: %s", err, scanner.Bytes())
			}
			var frame map[string]any
			if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
				t.Errorf("decode host frame: %v", err)
				return
			}
			frames <- frame
		}
	}()
	go manager.readRuntimeFrames(handle)
	t.Cleanup(func() {
		_ = stdoutWriter.Close()
		_ = stdinWriter.Close()
		_ = stdinReader.Close()
		if _, exited := handle.ExitResult(); !exited {
			handle.SetExit(nil)
		}
	})
	return &pipeRuntime{t: t, manager: manager, handle: handle, frames: frames, output: stdoutWriter, logs: logs, actions: actions}
}

func staticRuntimeConfig(runtimeConfig config.RuntimeConfig) func() config.RuntimeConfig {
	return func() config.RuntimeConfig { return runtimeConfig }
}

func (p *pipeRuntime) next() map[string]any {
	p.t.Helper()
	select {
	case frame, ok := <-p.frames:
		if !ok {
			p.t.Fatal("host closed the plugin input")
		}
		return frame
	case <-time.After(3 * time.Second):
		p.t.Fatal("host did not send a frame")
		return nil
	}
}

func (p *pipeRuntime) send(frame map[string]any) {
	p.t.Helper()
	encoded, err := json.Marshal(frame)
	if err != nil {
		p.t.Fatal(err)
	}
	if _, err := p.output.Write(append(encoded, '\n')); err != nil {
		p.t.Fatal(err)
	}
}

type deliveryOutcome struct {
	delivery plugins.Delivery
	err      error
}

// deliver starts a delivery and returns the event frame the plugin received.
func (p *pipeRuntime) deliver(ctx context.Context, event chatevent.Event) (<-chan deliveryOutcome, map[string]any) {
	p.t.Helper()
	done := make(chan deliveryOutcome, 1)
	go func() {
		delivery, err := p.manager.DeliverEvent(ctx, event)
		done <- deliveryOutcome{delivery: delivery, err: err}
	}()
	frame := p.next()
	if frame["type"] != "event" {
		p.t.Fatalf("expected an event frame, got %#v", frame)
	}
	return done, frame
}

// detach sends event.detach and returns the host response frame.
func (p *pipeRuntime) detach(eventRequestID, actionID string, data map[string]any) map[string]any {
	p.t.Helper()
	p.send(map[string]any{"type": "action", "request_id": actionID, "parent_request_id": eventRequestID, "action": "event.detach", "data": data})
	response := p.next()
	if response["request_id"] != actionID {
		p.t.Fatalf("detach response = %#v", response)
	}
	return response
}

func awaitDelivery(t *testing.T, done <-chan deliveryOutcome) deliveryOutcome {
	t.Helper()
	select {
	case outcome := <-done:
		return outcome
	case <-time.After(3 * time.Second):
		t.Fatal("delivery did not complete")
		return deliveryOutcome{}
	}
}

func awaitDetachedEnd(t *testing.T, detached *plugins.DetachedEvent) error {
	t.Helper()
	select {
	case <-detached.Done():
		return detached.Err()
	case <-time.After(5 * time.Second):
		t.Fatal("background event did not end")
		return nil
	}
}

func schedulerRuntimeEvent() chatevent.Event {
	return chatevent.Event{
		EventID: "scheduler-daily-1", SourceProtocol: "scheduler", SourceAdapter: "scheduler.internal",
		EventType: "scheduler.trigger", Timestamp: time.Now().Unix(),
		PayloadFields: map[string]any{"task_id": "daily", "payload": map[string]any{"scope": "all"}},
	}
}

func TestDetachCompletesDeliveryAndKeepsActingUntilTerminal(t *testing.T) {
	t.Parallel()
	p := startPipeRuntime(t, staticRuntimeConfig(config.RuntimeConfig{PluginDetachedEventTimeoutSeconds: 120}), nil)
	ctx, cancelDelivery := context.WithCancel(context.Background())
	before := time.Now()
	done, frame := p.deliver(ctx, testRuntimeEvent())
	eventID, _ := frame["request_id"].(string)
	deadline := int64(frame["deadline_at_ms"].(float64))
	if deadline < before.Add(5*time.Second).UnixMilli() || deadline > time.Now().Add(5*time.Second).UnixMilli() {
		t.Fatalf("frame deadline %d does not match the event timeout", deadline)
	}

	response := p.detach(eventID, "detach-1", map[string]any{"propagation": "stop", "result": map[string]any{"accepted": true}})
	data, _ := response["data"].(map[string]any)
	background := int64(data["deadline_at_ms"].(float64))
	if response["type"] != "result" || background < before.Add(120*time.Second).UnixMilli() || background > time.Now().Add(120*time.Second).UnixMilli() {
		t.Fatalf("detach response = %#v", response)
	}
	outcome := awaitDelivery(t, done)
	if outcome.err != nil || outcome.delivery.Detached == nil || outcome.delivery.Propagation != "stop" || outcome.delivery.Result["accepted"] != true {
		t.Fatalf("detached delivery = %#v, %v", outcome.delivery, outcome.err)
	}
	// Releasing the delivery context must not cancel the background event.
	cancelDelivery()

	p.send(map[string]any{"type": "action", "request_id": "log-1", "parent_request_id": eventID, "action": "logger.write", "data": map[string]any{"level": "info", "message": "progress"}})
	executed := <-p.actions
	if executed.kind != "logger.write" || executed.parent.EventID != "evt-1" || executed.ctx.Err() != nil {
		t.Fatalf("background action = %#v, ctx err %v", executed, executed.ctx.Err())
	}
	if response := p.next(); response["request_id"] != "log-1" || response["type"] != "result" {
		t.Fatalf("background action response = %#v", response)
	}
	select {
	case <-outcome.delivery.Detached.Done():
		t.Fatal("background event ended before its terminal")
	default:
	}
	p.send(map[string]any{"type": "result", "request_id": eventID, "status": "success", "data": map[string]any{"ignored": true}})
	if err := awaitDetachedEnd(t, outcome.delivery.Detached); err != nil {
		t.Fatalf("background terminal = %v", err)
	}

	var started, finished bool
	for _, record := range p.logs.records(t) {
		switch record["msg"] {
		case "插件fixture的事件已转入后台":
			started = record["event_type"] == "message.group" && record["deadline_at_ms"] == float64(background) && record["plain_text"] == nil
		case "插件fixture的后台事件已结束":
			finished = record["error_code"] == nil && record["duration_ms"] != nil
		}
	}
	if !started || !finished {
		t.Fatalf("background event logs are incomplete: %s", p.logs.buffer.String())
	}
}

func TestDetachReturnsManagementResultToCaller(t *testing.T) {
	t.Parallel()
	p := startPipeRuntime(t, staticRuntimeConfig(config.RuntimeConfig{}), nil)
	done, frame := p.deliver(context.Background(), chatevent.Event{
		EventID: "management-1", SourceProtocol: "management", SourceAdapter: "management.ui",
		EventType: "management.action", Timestamp: time.Now().Unix(), PayloadFields: map[string]any{"action": "refresh"},
	})
	eventID := frame["request_id"].(string)
	p.detach(eventID, "detach-1", map[string]any{"result": map[string]any{"task": "started"}})
	outcome := awaitDelivery(t, done)
	if outcome.err != nil || outcome.delivery.Result["task"] != "started" || outcome.delivery.Detached == nil {
		t.Fatalf("management delivery = %#v, %v", outcome.delivery, outcome.err)
	}
	p.send(map[string]any{"type": "error", "request_id": eventID, "code": "plugin.internal_error", "message": "refresh failed"})
	err := awaitDetachedEnd(t, outcome.delivery.Detached)
	assertRuntimeErrorCode(t, err, codePluginInternalError)
}

func TestDetachRejectionsKeepTheEventInTheForeground(t *testing.T) {
	t.Parallel()
	webhook := chatevent.Event{
		EventID: "webhook-1", SourceProtocol: "webhook", SourceAdapter: "webhook.internal", EventType: "webhook.received",
		Timestamp: time.Now().Unix(), Webhook: &chatevent.Webhook{Route: "hook", ReceivedAt: time.Now().Unix()},
	}
	for name, test := range map[string]struct {
		event chatevent.Event
		data  map[string]any
	}{
		"event type":  {event: webhook},
		"propagation": {event: schedulerRuntimeEvent(), data: map[string]any{"propagation": "continue"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := startPipeRuntime(t, staticRuntimeConfig(config.RuntimeConfig{}), nil)
			done, frame := p.deliver(context.Background(), test.event)
			eventID := frame["request_id"].(string)
			data := test.data
			if data == nil {
				data = map[string]any{}
			}
			if response := p.detach(eventID, "detach-1", data); response["type"] != "error" || response["code"] != codePlatformInvalidRequest {
				t.Fatalf("rejected detach response = %#v", response)
			}
			p.send(map[string]any{"type": "result", "request_id": eventID, "status": "success", "data": map[string]any{"done": true}})
			outcome := awaitDelivery(t, done)
			if outcome.err != nil || outcome.delivery.Detached != nil || outcome.delivery.Result["done"] != true {
				t.Fatalf("foreground delivery = %#v, %v", outcome.delivery, outcome.err)
			}
		})
	}
}

func TestDetachIsOncePerEventAndRejectsSessionWait(t *testing.T) {
	t.Parallel()
	p := startPipeRuntime(t, staticRuntimeConfig(config.RuntimeConfig{}), nil)
	done, frame := p.deliver(context.Background(), testRuntimeEvent())
	eventID := frame["request_id"].(string)
	p.detach(eventID, "detach-1", map[string]any{})
	outcome := awaitDelivery(t, done)
	if response := p.detach(eventID, "detach-2", map[string]any{}); response["code"] != codePlatformInvalidRequest {
		t.Fatalf("second detach response = %#v", response)
	}
	p.send(map[string]any{"type": "action", "request_id": "wait-1", "parent_request_id": eventID, "action": "session.wait", "data": map[string]any{}})
	if response := p.next(); response["request_id"] != "wait-1" || response["code"] != codePlatformInvalidRequest {
		t.Fatalf("session.wait response = %#v", response)
	}
	select {
	case executed := <-p.actions:
		t.Fatalf("rejected action reached the executor: %#v", executed)
	default:
	}
	p.send(map[string]any{"type": "result", "request_id": eventID, "status": "success", "data": map[string]any{}})
	if err := awaitDetachedEnd(t, outcome.delivery.Detached); err != nil {
		t.Fatal(err)
	}
}

func TestDetachCapIsPerProcessAndRetryable(t *testing.T) {
	t.Parallel()
	p := startPipeRuntime(t, staticRuntimeConfig(config.RuntimeConfig{MaxDetachedEventsPerPlugin: 1}), nil)
	firstDone, first := p.deliver(context.Background(), testRuntimeEventWithTarget("1"))
	p.detach(first["request_id"].(string), "detach-1", map[string]any{})
	firstOutcome := awaitDelivery(t, firstDone)

	secondDone, second := p.deliver(context.Background(), testRuntimeEventWithTarget("2"))
	secondID := second["request_id"].(string)
	response := p.detach(secondID, "detach-2", map[string]any{})
	details, _ := response["details"].(map[string]any)
	if response["code"] != codePlatformRateLimited || details["limit"] != float64(1) {
		t.Fatalf("capped detach response = %#v", response)
	}
	p.send(map[string]any{"type": "result", "request_id": secondID, "status": "success", "data": map[string]any{}})
	if outcome := awaitDelivery(t, secondDone); outcome.err != nil || outcome.delivery.Detached != nil {
		t.Fatalf("capped event did not stay in the foreground: %#v, %v", outcome.delivery, outcome.err)
	}

	p.send(map[string]any{"type": "result", "request_id": first["request_id"], "status": "success", "data": map[string]any{}})
	if err := awaitDetachedEnd(t, firstOutcome.delivery.Detached); err != nil {
		t.Fatal(err)
	}
	thirdDone, third := p.deliver(context.Background(), testRuntimeEventWithTarget("3"))
	if response := p.detach(third["request_id"].(string), "detach-3", map[string]any{}); response["type"] != "result" {
		t.Fatalf("detach after a background event ended = %#v", response)
	}
	awaitDelivery(t, thirdDone)
}

func TestDetachedEventRejectsForegroundTerminals(t *testing.T) {
	t.Parallel()
	for name, terminal := range map[string]func(string) map[string]any{
		"terminal action": func(eventID string) map[string]any {
			return map[string]any{"type": "action", "request_id": eventID, "action": "message.send", "data": map[string]any{
				"target_type": "group", "target_id": "2001", "message": map[string]any{"segments": []any{map[string]any{"type": "text", "data": map[string]any{"text": "done"}}}},
			}}
		},
		"propagation": func(eventID string) map[string]any {
			return map[string]any{"type": "result", "request_id": eventID, "status": "success", "data": map[string]any{}, "propagation": "continue"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := startPipeRuntime(t, staticRuntimeConfig(config.RuntimeConfig{}), nil)
			done, frame := p.deliver(context.Background(), testRuntimeEvent())
			eventID := frame["request_id"].(string)
			p.detach(eventID, "detach-1", map[string]any{})
			outcome := awaitDelivery(t, done)
			p.send(terminal(eventID))
			assertRuntimeErrorCode(t, awaitDetachedEnd(t, outcome.delivery.Detached), codePluginProtocolViolation)
		})
	}
}

func TestDetachedEventTimeoutEndsEventAndFailsServiceCalls(t *testing.T) {
	t.Parallel()
	calls := make(chan context.Context, 1)
	executed := make(chan string, 4)
	p := startPipeRuntime(t, staticRuntimeConfig(config.RuntimeConfig{PluginDetachedEventTimeoutSeconds: 1}), func(ctx context.Context, _ string, _ string, action plugins.Action, _ chatevent.Event) (map[string]any, error) {
		executed <- action.Kind
		if action.Kind == "plugin.call" {
			calls <- ctx
			<-ctx.Done()
			return nil, eventContextError(ctx.Err())
		}
		return map[string]any{}, nil
	})
	done, frame := p.deliver(context.Background(), schedulerRuntimeEvent())
	eventID := frame["request_id"].(string)
	p.detach(eventID, "detach-1", map[string]any{})
	outcome := awaitDelivery(t, done)
	p.send(map[string]any{"type": "action", "request_id": "call-1", "parent_request_id": eventID, "action": "plugin.call", "data": map[string]any{
		"target_plugin_id": "provider", "service": "accounts", "service_version": 1, "method": "read", "params": map[string]any{},
	}})
	<-calls
	<-executed

	err := awaitDetachedEnd(t, outcome.delivery.Detached)
	assertRuntimeErrorCode(t, err, codePluginEventTimeout)
	var failure *plugins.Error
	if !errors.As(err, &failure) || !failure.FailureReported() {
		t.Fatal("the runtime did not own the timeout record")
	}
	if response := p.next(); response["request_id"] != "call-1" || response["code"] != codePluginEventTimeout {
		t.Fatalf("pending service call response = %#v", response)
	}

	// Later actions and the late terminal of the expired event are ignored.
	p.send(map[string]any{"type": "action", "request_id": "log-late", "parent_request_id": eventID, "action": "logger.write", "data": map[string]any{"level": "info", "message": "late"}})
	p.send(map[string]any{"type": "result", "request_id": eventID, "status": "success", "data": map[string]any{}})
	nextDone, next := p.deliver(context.Background(), testRuntimeEvent())
	p.send(map[string]any{"type": "result", "request_id": next["request_id"], "status": "success", "data": map[string]any{}})
	if outcome := awaitDelivery(t, nextDone); outcome.err != nil {
		t.Fatalf("runtime failed after an ignored late frame: %v", outcome.err)
	}
	select {
	case kind := <-executed:
		t.Fatalf("expired event executed %s", kind)
	default:
	}

	var warned bool
	for _, record := range p.logs.records(t) {
		if record["level"] == "WARN" && record["error_code"] == codePluginEventTimeout && record["task_id"] == "daily" {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("missing timeout warning: %s", p.logs.buffer.String())
	}
}

func TestStopCancelsDetachedEventsWithoutWaiting(t *testing.T) {
	t.Parallel()
	p := startPipeRuntime(t, staticRuntimeConfig(config.RuntimeConfig{}), nil)
	done, frame := p.deliver(context.Background(), schedulerRuntimeEvent())
	p.detach(frame["request_id"].(string), "detach-1", map[string]any{})
	outcome := awaitDelivery(t, done)

	stopped := make(chan error, 1)
	go func() { stopped <- p.manager.Stop(context.Background()) }()
	assertRuntimeErrorCode(t, awaitDetachedEnd(t, outcome.delivery.Detached), codePluginEventCanceled)
	if shutdown := p.next(); shutdown["type"] != "shutdown" {
		t.Fatalf("expected shutdown, got %#v", shutdown)
	}
	p.handle.SetExit(nil)
	_ = p.output.Close()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stop waited for the background event")
	}
}

func TestEventFrameDeadlineFollowsServiceDeadline(t *testing.T) {
	t.Parallel()
	p := startPipeRuntime(t, staticRuntimeConfig(config.RuntimeConfig{}), nil)
	deadline := time.Now().Add(2 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	request := pluginwire.ProtocolServiceRequestFrame{
		CallerPluginID: "caller", Service: "accounts", ServiceVersion: 1, Method: "read", Params: map[string]any{},
		DeadlineAtMs: deadline.UnixMilli(), Origin: pluginwire.ProtocolServiceOriginFrame{EventID: "origin", EventType: "message.group", SourceProtocol: "onebot11", SourceAdapter: "onebot"},
	}
	done, frame := p.deliver(ctx, chatevent.Event{
		EventID: "service-1", SourceProtocol: "platform", SourceAdapter: "plugins.internal", EventType: "plugin.request",
		Timestamp: time.Now().UnixMilli(), PayloadFields: map[string]any{"service_request": request},
	})
	if int64(frame["deadline_at_ms"].(float64)) != deadline.UnixMilli() {
		t.Fatalf("service event deadline = %v, want %d", frame["deadline_at_ms"], deadline.UnixMilli())
	}
	eventID := frame["request_id"].(string)
	if response := p.detach(eventID, "detach-1", map[string]any{}); response["code"] != codePlatformInvalidRequest {
		t.Fatalf("service request detached: %#v", response)
	}
	p.send(map[string]any{"type": "result", "request_id": eventID, "status": "success", "data": map[string]any{}})
	awaitDelivery(t, done)
}

func TestParseEventDetachRejectsMalformedData(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`{"result":null}`, `{"result":[]}`, `{"propagation":"skip"}`, `{"timeout_seconds":60}`} {
		if _, err := ParseLocalAction("event.detach", json.RawMessage(raw)); err == nil || !strings.Contains(err.Error(), "event.detach") {
			t.Fatalf("malformed detach data %s accepted: %v", raw, err)
		}
	}
	action, err := ParseLocalAction("event.detach", json.RawMessage(`{"result":{"a":1},"propagation":"continue"}`))
	if err != nil || action.DetachResult["a"] != float64(1) || action.DetachPropagation != "continue" {
		t.Fatalf("detach data = %#v, %v", action, err)
	}
}

func TestDetachReadsLimitsSavedAfterStart(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	current := config.RuntimeConfig{PluginDetachedEventTimeoutSeconds: 120}
	p := startPipeRuntime(t, func() config.RuntimeConfig {
		mu.Lock()
		defer mu.Unlock()
		return current
	}, nil)
	detachDeadline := func(target string) (int64, *plugins.DetachedEvent) {
		done, frame := p.deliver(context.Background(), testRuntimeEventWithTarget(target))
		response := p.detach(frame["request_id"].(string), "detach-"+target, map[string]any{})
		data, _ := response["data"].(map[string]any)
		return int64(data["deadline_at_ms"].(float64)), awaitDelivery(t, done).delivery.Detached
	}
	first, firstEvent := detachDeadline("1")
	mu.Lock()
	current = config.RuntimeConfig{PluginDetachedEventTimeoutSeconds: 1800}
	mu.Unlock()
	second, _ := detachDeadline("2")
	if second-first < (1800-120-5)*1000 {
		t.Fatalf("saved timeout did not apply to the next detach: %d then %d", first, second)
	}
	select {
	case <-firstEvent.Done():
		t.Fatal("changing the limits ended an existing background event")
	default:
	}
}
