package rayleabot

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func writeEventWithDeadline(t *testing.T, peer *sessionPeer, requestID string, event Event, deadline time.Time) {
	t.Helper()
	raw := map[string]any{"event_id": event.EventID, "source_protocol": event.SourceProtocol, "source_adapter": event.SourceAdapter, "event_type": event.EventType, "timestamp": event.Timestamp}
	if event.Target.ID != "" {
		raw["target"] = event.Target
	}
	if event.Payload != nil {
		raw["payload"] = event.Payload
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	writeFrame(t, peer.encoder, protocolFrame{Type: "event", RequestID: requestID, DeadlineAtMs: deadline.UnixMilli(), Event: payload})
}

type detachObservation struct {
	before, after, reported time.Time
	contextErr              error
	sendErr                 error
}

func TestDetachExtendsTheHandlerContextAndRepliesWithActionAndResult(t *testing.T) {
	observed := make(chan detachObservation, 1)
	release := make(chan struct{})
	peer := newSessionPeer(t, func(ctx context.Context, event *EventContext) error {
		if event.Event.EventID != "long" {
			return nil
		}
		var observation detachObservation
		observation.before, _ = ctx.Deadline()
		deadline, err := event.Detach(ctx, map[string]any{"accepted": true})
		if err != nil {
			return err
		}
		observation.after, _ = ctx.Deadline()
		observation.reported = event.Deadline()
		if !deadline.Equal(observation.reported) {
			t.Errorf("Detach returned %v, Deadline reports %v", deadline, observation.reported)
		}
		<-release
		observation.contextErr = ctx.Err()
		observation.sendErr = event.SendText("done")
		observed <- observation
		return nil
	})
	eventDeadline := time.Now().Add(150 * time.Millisecond)
	writeEventWithDeadline(t, peer, "long", sessionMessage("long", nil), eventDeadline)
	detach := peer.read(t)
	var data map[string]any
	_ = json.Unmarshal(detach.Data, &data)
	if detach.Action != "event.detach" || detach.ParentRequestID != "long" || data["result"].(map[string]any)["accepted"] != true || data["propagation"] != nil {
		t.Fatalf("detach frame = %#v", detach)
	}
	background := time.Now().Add(10 * time.Second).Truncate(time.Millisecond)
	peer.reply(t, detach.RequestID, map[string]any{"deadline_at_ms": background.UnixMilli()})

	// The detached handler no longer holds the only concurrency permit.
	writeRuntimeEvent(t, peer.encoder, "ordinary", sessionMessage("ordinary", nil))
	if frame := peer.read(t); frame.Type != "result" || frame.RequestID != "ordinary" {
		t.Fatalf("ordinary event waited for the background event: %#v", frame)
	}
	time.Sleep(time.Until(eventDeadline) + 50*time.Millisecond)
	close(release)

	send := peer.read(t)
	if send.Type != "action" || send.Action != "message.send" || send.ParentRequestID != "long" || send.RequestID == "long" {
		t.Fatalf("background reply was not an ordinary action: %#v", send)
	}
	// An ordinary message action names the event's adapter, which a terminal
	// reply would have used implicitly.
	var sent map[string]any
	_ = json.Unmarshal(send.Data, &sent)
	if sent["source_protocol"] != "onebot11" || sent["source_adapter"] != "adapter" || sent["target_type"] != "group" || sent["target_id"] != "group" {
		t.Fatalf("background reply data = %s", send.Data)
	}
	peer.reply(t, send.RequestID, map[string]any{"message_id": "sent"})
	if terminal := peer.read(t); terminal.Type != "result" || terminal.RequestID != "long" || terminal.Propagation != "" {
		t.Fatalf("background terminal = %#v", terminal)
	}
	observation := <-observed
	if !observation.before.Equal(time.UnixMilli(eventDeadline.UnixMilli())) || !observation.after.Equal(background) || observation.contextErr != nil || observation.sendErr != nil {
		t.Fatalf("observation = %#v", observation)
	}
	peer.stop(t)
}

func TestHandlerContextEndsAtTheEventDeadline(t *testing.T) {
	ended := make(chan error, 1)
	peer := newSessionPeer(t, func(ctx context.Context, event *EventContext) error {
		if event.Event.EventID != "slow" {
			return nil
		}
		<-ctx.Done()
		ended <- ctx.Err()
		return ctx.Err()
	})
	writeEventWithDeadline(t, peer, "slow", sessionMessage("slow", nil), time.Now().Add(100*time.Millisecond))
	select {
	case err := <-ended:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("handler context ended with %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler context outlived the event deadline")
	}
	// No terminal follows an expired event: the next frame answers the barrier.
	writeFrame(t, peer.encoder, protocolFrame{Type: "ping", RequestID: "barrier"})
	if frame := peer.read(t); frame.Type != "pong" {
		t.Fatalf("expired event sent %#v", frame)
	}
	peer.stop(t)
}

func TestDetachedContextEndsAtTheBackgroundDeadline(t *testing.T) {
	ended := make(chan time.Time, 1)
	peer := newSessionPeer(t, func(ctx context.Context, event *EventContext) error {
		if _, err := event.Detach(ctx, nil); err != nil {
			return err
		}
		<-ctx.Done()
		ended <- time.Now()
		return ctx.Err()
	})
	writeEventWithDeadline(t, peer, "task", Event{EventID: "task", SourceProtocol: "scheduler", SourceAdapter: "scheduler.internal", EventType: "scheduler.trigger", Payload: map[string]any{"task_id": "daily"}}, time.Now().Add(time.Minute))
	detach := peer.read(t)
	if string(detach.Data) != "{}" {
		t.Fatalf("nil detach result sent %s", detach.Data)
	}
	background := time.Now().Add(150 * time.Millisecond)
	peer.reply(t, detach.RequestID, map[string]any{"deadline_at_ms": background.UnixMilli()})
	select {
	case at := <-ended:
		if at.Before(time.UnixMilli(background.UnixMilli())) {
			t.Fatalf("context ended at %v before the background deadline %v", at, background)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("background context outlived its deadline")
	}
	peer.stop(t)
}

func TestRejectedDetachKeepsTheEventInTheForeground(t *testing.T) {
	failures := make(chan error, 1)
	peer := newSessionPeer(t, func(ctx context.Context, event *EventContext) error {
		_, err := event.DetachWithPropagation(ctx, nil, PropagationStop)
		failures <- err
		return event.SendText("foreground")
	})
	writeRuntimeEvent(t, peer.encoder, "busy", sessionMessage("busy", nil))
	detach := peer.read(t)
	var data map[string]any
	_ = json.Unmarshal(detach.Data, &data)
	if data["propagation"] != "stop" {
		t.Fatalf("detach frame = %s", detach.Data)
	}
	writeFrame(t, peer.encoder, protocolFrame{Type: "error", RequestID: detach.RequestID, Code: "platform.rate_limited", Message: "background event limit reached", Details: map[string]any{"limit": 8}})
	var failure *ActionError
	if err := <-failures; !errors.As(err, &failure) || failure.Code != "platform.rate_limited" {
		t.Fatalf("rejected detach error = %v", err)
	}
	if terminal := peer.read(t); terminal.Type != "action" || terminal.RequestID != "busy" || terminal.Action != "message.send" {
		t.Fatalf("foreground reply = %#v", terminal)
	}
	peer.stop(t)
}

func TestDetachValidatesArgumentsBeforeSending(t *testing.T) {
	event := &EventContext{Event: Event{EventType: "scheduler.trigger"}}
	if _, err := event.DetachWithPropagation(context.Background(), nil, PropagationStop); err == nil {
		t.Fatal("propagation accepted for a scheduler trigger")
	}
	event.Event.EventType = "message.group"
	if _, err := event.Detach(context.Background(), "text"); err == nil {
		t.Fatal("non-object detach result accepted")
	}
	event.detached.Store(true)
	if _, err := event.Detach(context.Background(), nil); err == nil {
		t.Fatal("second detach accepted")
	}
	if err := event.ResultWithPropagation(nil, PropagationContinue); err == nil {
		t.Fatal("detached event decided propagation again")
	}
}

func TestEventFrameWithoutDeadlineIsRejected(t *testing.T) {
	peer := newSessionPeer(t, func(context.Context, *EventContext) error {
		t.Error("handler ran for an event without a deadline")
		return nil
	})
	payload, _ := json.Marshal(map[string]any{"event_id": "missing", "source_protocol": "platform", "source_adapter": "plugins.internal", "event_type": "plugin.started", "timestamp": 1})
	if err := peer.encoder.Encode(map[string]any{"type": "event", "request_id": "missing", "event": json.RawMessage(payload)}); err != nil {
		t.Fatal(err)
	}
	if frame := peer.read(t); frame.Type != "error" || frame.RequestID != "missing" || frame.Code != "plugin.protocol_violation" {
		t.Fatalf("event without deadline = %#v", frame)
	}
	peer.stop(t)
}
