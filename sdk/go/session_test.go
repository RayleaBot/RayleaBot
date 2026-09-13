package rayleabot

import (
	"context"
	"encoding/json"
	"io"
	"sync/atomic"
	"testing"
	"time"
)

type sessionPeer struct {
	encoder *json.Encoder
	decoder *json.Decoder
	done    chan error
}

func newSessionPeer(t *testing.T, handler HandlerFunc) *sessionPeer {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	t.Cleanup(func() { _ = inR.Close(); _ = inW.Close(); _ = outR.Close(); _ = outW.Close() })
	peer := &sessionPeer{encoder: json.NewEncoder(inW), decoder: json.NewDecoder(outR), done: make(chan error, 1)}
	go func() {
		peer.done <- Run(t.Context(), Options{Stdin: inR, Stdout: outW, Stderr: io.Discard, ActionTimeout: time.Second}, handler)
	}()
	writeFrame(t, peer.encoder, protocolFrame{Type: "init", RequestID: "init", ProtocolVersion: ProtocolVersion, PluginID: "fixture", Timezone: "UTC", Concurrency: 1, Bots: &[]Bot{{SourceProtocol: "onebot11", SourceAdapter: "adapter", ID: "bot"}}})
	var ack protocolFrame
	decodeFrame(t, peer.decoder, &ack)
	return peer
}
func (p *sessionPeer) reply(t *testing.T, requestID string, data any) {
	t.Helper()
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	writeFrame(t, p.encoder, protocolFrame{Type: "result", RequestID: requestID, Status: "success", Data: encoded})
}
func (p *sessionPeer) read(t *testing.T) protocolFrame {
	t.Helper()
	var frame protocolFrame
	decodeFrame(t, p.decoder, &frame)
	return frame
}
func (p *sessionPeer) stop(t *testing.T) {
	t.Helper()
	writeFrame(t, p.encoder, protocolFrame{Type: "shutdown", RequestID: "shutdown", Reason: "stop"})
	select {
	case err := <-p.done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SDK did not stop")
	}
}
func sessionMessage(id string, ref *SessionRef) Event {
	event := Event{EventID: id, SourceProtocol: "onebot11", SourceAdapter: "adapter", EventType: "message.group", Actor: Actor{ID: "actor"}, Target: Target{Type: "group", ID: "group"}, Message: Message{PlainText: id}}
	if ref != nil {
		event.Payload = map[string]any{"session": *ref}
	}
	return event
}

func TestAskThreeRoundsAtConcurrencyOne(t *testing.T) {
	var ordinary, calls atomic.Int32
	var previous *EventContext
	var next HandlerFunc
	next = func(ctx context.Context, event *EventContext) error {
		if event == previous || event.RequestID == previous.RequestID {
			t.Error("callback retained old event")
		}
		if _, err := previous.Actions().LoggerWrite(ctx, LoggerWriteRequest{Level: "info", Message: "late"}); err == nil {
			t.Error("old event accepted an action")
		}
		previous = event
		if calls.Add(1) == 3 {
			return event.SendText("done")
		}
		_, err := event.Ask(ctx, "next", SessionWaitOptions{}, next)
		return err
	}
	peer := newSessionPeer(t, func(ctx context.Context, event *EventContext) error {
		ordinary.Add(1)
		if event.Event.EventID != "initial" {
			return nil
		}
		previous = event
		_, err := event.Ask(ctx, "role", SessionWaitOptions{}, next)
		return err
	})
	var ref SessionRef
	for round := 0; round < 4; round++ {
		id := "initial"
		var session *SessionRef
		if round > 0 {
			id = string(rune('a' + round))
			session = &ref
		}
		writeRuntimeEvent(t, peer.encoder, id, sessionMessage(id, session))
		if round == 3 {
			frame := peer.read(t)
			if frame.Type != "action" || frame.RequestID != id || frame.Action != "message.send" {
				t.Fatalf("final=%#v", frame)
			}
			break
		}
		wait := peer.read(t)
		if wait.Action != "session.wait" || wait.ParentRequestID != id {
			t.Fatalf("wait=%#v", wait)
		}
		var request map[string]any
		_ = json.Unmarshal(wait.Data, &request)
		if round > 0 && request["session_id"] != ref.SessionID {
			t.Fatal("callback did not reuse conversation ID")
		}
		if request["revision"] != nil || request["state"] != nil {
			t.Fatal("SDK transmitted removed state fields")
		}
		ref = SessionRef{SessionID: "session", Scope: "user", ExpiresAtMS: time.Now().Add(time.Minute).UnixMilli()}
		peer.reply(t, wait.RequestID, ref)
		prompt := peer.read(t)
		if prompt.Action != "message.send" || prompt.ParentRequestID != id || prompt.RequestID == id {
			t.Fatal("prompt was terminal or retained wrong parent")
		}
		peer.reply(t, prompt.RequestID, map[string]any{"message_id": "prompt"})
		terminal := peer.read(t)
		if terminal.Type != "result" || terminal.RequestID != id {
			t.Fatalf("terminal=%#v", terminal)
		}
	}
	writeRuntimeEvent(t, peer.encoder, "ordinary", sessionMessage("ordinary", nil))
	if frame := peer.read(t); frame.Type != "result" || frame.RequestID != "ordinary" {
		t.Fatal("callback leaked an execution permit")
	}
	peer.stop(t)
	if ordinary.Load() != 2 || calls.Load() != 3 {
		t.Fatalf("ordinary=%d callbacks=%d", ordinary.Load(), calls.Load())
	}
}

func TestAskPromptFailureFinishesHostAndDropsLateReply(t *testing.T) {
	var ordinary, callbacks atomic.Int32
	peer := newSessionPeer(t, func(ctx context.Context, event *EventContext) error {
		ordinary.Add(1)
		_, err := event.Ask(ctx, "prompt", SessionWaitOptions{}, func(context.Context, *EventContext) error { callbacks.Add(1); return nil })
		return err
	})
	writeRuntimeEvent(t, peer.encoder, "initial", sessionMessage("initial", nil))
	wait := peer.read(t)
	ref := SessionRef{SessionID: "failed-prompt", Scope: "user", ExpiresAtMS: time.Now().Add(time.Minute).UnixMilli()}
	peer.reply(t, wait.RequestID, ref)
	prompt := peer.read(t)
	writeFrame(t, peer.encoder, protocolFrame{Type: "error", RequestID: prompt.RequestID, Code: "adapter.send_failed", Message: "fixture send failure"})
	finish := peer.read(t)
	if finish.Action != "session.finish" || finish.ParentRequestID != "initial" {
		t.Fatalf("cleanup=%#v", finish)
	}
	peer.reply(t, finish.RequestID, map[string]any{"finished": true})
	if frame := peer.read(t); frame.Type != "error" || frame.RequestID != "initial" {
		t.Fatal("prompt failure was swallowed")
	}
	writeRuntimeEvent(t, peer.encoder, "late", sessionMessage("late", &ref))
	if frame := peer.read(t); frame.Type != "result" || frame.RequestID != "late" {
		t.Fatal("late callback event was not closed")
	}
	peer.stop(t)
	if ordinary.Load() != 1 || callbacks.Load() != 0 {
		t.Fatal("failed prompt resumed a callback or ordinary handler")
	}
}

func TestCallbackTimeoutDoesNotDependOnHostNotification(t *testing.T) {
	store := &sessionCallbacks{}
	if err := store.reserve(""); err != nil {
		t.Fatal(err)
	}
	ref := SessionRef{SessionID: "timeout", Scope: "user", ExpiresAtMS: time.Now().Add(time.Minute).UnixMilli()}
	if err := store.install(ref, "route", time.Now().Add(20*time.Millisecond), func(context.Context, *EventContext) error { t.Error("expired callback invoked"); return nil }); err != nil {
		t.Fatal(err)
	}
	store.release()
	deadline := time.After(time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		store.mu.Lock()
		remaining := len(store.waiting)
		store.mu.Unlock()
		if remaining == 0 {
			break
		}
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatal("missing notification retained callback")
		}
	}
	if handler, owned := store.take(ref); handler != nil || !owned {
		t.Fatal("timed-out reply could reach ordinary handler")
	}
	store.close()
}

func TestExpiredCallbackInstallationStillOwnsLateInput(t *testing.T) {
	store := &sessionCallbacks{}
	if err := store.reserve(""); err != nil {
		t.Fatal(err)
	}
	ref := SessionRef{SessionID: "late-install", Scope: "user", ExpiresAtMS: time.Now().Add(time.Minute).UnixMilli()}
	if err := store.install(ref, "route", time.Now().Add(-time.Second), func(context.Context, *EventContext) error { return nil }); err == nil {
		t.Fatal("expired installation succeeded")
	}
	store.release()
	if handler, owned := store.take(ref); handler != nil || !owned {
		t.Fatal("aborted callback input reached ordinary handler")
	}
	store.close()
}

func TestCallbackPanicClosesNewRequestAndReleasesPermit(t *testing.T) {
	var ordinary atomic.Int32
	peer := newSessionPeer(t, func(ctx context.Context, event *EventContext) error {
		ordinary.Add(1)
		if event.Event.EventID != "initial" {
			return nil
		}
		_, err := event.Ask(ctx, "prompt", SessionWaitOptions{}, func(context.Context, *EventContext) error { panic("fixture panic") })
		return err
	})
	writeRuntimeEvent(t, peer.encoder, "initial", sessionMessage("initial", nil))
	wait := peer.read(t)
	ref := SessionRef{SessionID: "panic", Scope: "user", ExpiresAtMS: time.Now().Add(time.Minute).UnixMilli()}
	peer.reply(t, wait.RequestID, ref)
	prompt := peer.read(t)
	peer.reply(t, prompt.RequestID, map[string]any{"message_id": "prompt"})
	peer.read(t)
	writeRuntimeEvent(t, peer.encoder, "reply", sessionMessage("reply", &ref))
	if frame := peer.read(t); frame.Type != "error" || frame.RequestID != "reply" {
		t.Fatal("panic closed the wrong request")
	}
	writeRuntimeEvent(t, peer.encoder, "ordinary", sessionMessage("ordinary", nil))
	if frame := peer.read(t); frame.Type != "result" || frame.RequestID != "ordinary" {
		t.Fatal("panic leaked permit")
	}
	peer.stop(t)
	if ordinary.Load() != 2 {
		t.Fatal("callback was routed through ordinary handler")
	}
}

func TestCallbackReservationBoundAndRawRegistration(t *testing.T) {
	store := &sessionCallbacks{}
	for range sessionCallbackLimit {
		if err := store.reserve(""); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.reserve(""); err == nil {
		t.Fatal("callback reservations unbounded")
	}
	for range sessionCallbackLimit {
		store.release()
	}
	ref := SessionRef{SessionID: "registered", Scope: "user", ExpiresAtMS: time.Now().Add(time.Minute).UnixMilli()}
	if err := store.install(ref, "route", time.Now().Add(time.Minute), func(context.Context, *EventContext) error { return nil }); err != nil {
		t.Fatal(err)
	}
	store.registered(ref, "route")
	if handler, owned := store.take(ref); handler != nil || owned {
		t.Fatal("raw registration was still captured by callback cache")
	}
	store.close()
}
