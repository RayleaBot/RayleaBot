package rayleabot

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type actionFrameSink chan protocolFrame

func TestEventAddressUsesWireAccountAndTarget(t *testing.T) {
	for _, tc := range []struct {
		name     string
		protocol string
		botID    string
		target   Target
		valid    bool
	}{
		{name: "OneBot", protocol: "onebot11", botID: "10001", target: Target{Type: "group", ID: "200"}, valid: true},
		{name: "QQ official", protocol: "qqofficial", botID: "qq-user-id", target: Target{Type: "private", ID: "openid"}, valid: true},
		{name: "platform", protocol: "platform", botID: "10001", target: Target{Type: "group", ID: "200"}},
		{name: "unknown protocol", protocol: "unknown", botID: "10001", target: Target{Type: "group", ID: "200"}},
		{name: "unknown account", protocol: "onebot11", target: Target{Type: "group", ID: "200"}},
		{name: "no target", protocol: "onebot11", botID: "10001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{"source_protocol": tc.protocol, "source_adapter": "replaceable", "bot_id": tc.botID, "target": tc.target})
			if err != nil {
				t.Fatal(err)
			}
			state := &runtimeState{}
			decoded, err := state.decodeEvent(protocolFrame{DeadlineAtMs: time.Now().Add(time.Minute).UnixMilli(), Event: raw})
			if err != nil {
				t.Fatal(err)
			}
			address, ok := (&EventContext{Event: decoded}).Address()
			want := Address{}
			if tc.valid {
				want = Address{SourceProtocol: tc.protocol, BotID: tc.botID, TargetType: tc.target.Type, TargetID: tc.target.ID}
			}
			if ok != tc.valid || address != want {
				t.Fatalf("Address() = %+v, %v; want %+v, %v", address, ok, want, tc.valid)
			}
		})
	}
}

func TestSendToEmitsPersistentAddress(t *testing.T) {
	sink := make(actionFrameSink, 1)
	client := newRuntimeClient(sink, time.Second)
	event := &EventContext{RequestID: "platform-event", client: client}
	done := make(chan error, 1)
	go func() {
		_, err := event.Actions().SendTo(t.Context(), Address{SourceProtocol: "onebot11", BotID: "10001", TargetType: "group", TargetID: "200"}, Text("hello"))
		done <- err
	}()
	var frame protocolFrame
	select {
	case frame = <-sink:
	case <-time.After(time.Second):
		t.Fatal("action frame not emitted")
	}
	var data map[string]any
	if err := json.Unmarshal(frame.Data, &data); err != nil {
		t.Fatal(err)
	}
	if frame.Action != "message.send" || frame.ParentRequestID != event.RequestID || data["source_protocol"] != "onebot11" || data["bot_id"] != "10001" || data["target_type"] != "group" || data["target_id"] != "200" {
		t.Fatalf("frame=%+v data=%v", frame, data)
	}
	if _, present := data["source_adapter"]; present {
		t.Fatal("SendTo carried an instance selector")
	}
	var request MessageSendRequest
	if err := json.Unmarshal(frame.Data, &request); err != nil {
		t.Fatal(err)
	}
	if len(request.Message.Segments) != 1 || request.Message.Segments[0].Data["text"] != "hello" {
		t.Fatalf("message=%+v", request.Message)
	}
	if !client.routeResponse(protocolFrame{Type: "result", RequestID: frame.RequestID}) {
		t.Fatal("response not routed")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestMessageSendCarriesConfiguredAdapterInstance(t *testing.T) {
	sink := make(actionFrameSink, 1)
	client := newRuntimeClient(sink, time.Second)
	event := &EventContext{RequestID: "event-1", client: client}
	done := make(chan error, 1)
	go func() {
		_, err := event.Actions().MessageSend(context.Background(), MessageSendRequest{SourceAdapter: "second-bot", SourceProtocol: "onebot11", TargetType: "group", TargetID: "301", Message: MessageOut{Segments: []Segment{{Type: "text", Data: map[string]any{"text": "fixture"}}}}})
		done <- err
	}()
	var frame protocolFrame
	select {
	case frame = <-sink:
	case <-time.After(time.Second):
		t.Fatal("action frame not emitted")
	}
	var data map[string]any
	if err := json.Unmarshal(frame.Data, &data); err != nil {
		t.Fatal(err)
	}
	if data["source_adapter"] != "second-bot" || data["source_protocol"] != "onebot11" {
		t.Fatalf("routing data=%v", data)
	}
	if !client.routeResponse(protocolFrame{Type: "result", RequestID: frame.RequestID}) {
		t.Fatal("response not routed")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func (sink actionFrameSink) Write(data []byte) (int, error) {
	var frame protocolFrame
	if err := json.Unmarshal(data, &frame); err != nil {
		return 0, err
	}
	sink <- frame
	return len(data), nil
}

func TestCanceledActionWaitDrainsBeforeTerminal(t *testing.T) {
	sink := make(actionFrameSink, 8)
	client := newRuntimeClient(sink, time.Second)
	event := &EventContext{RequestID: "event-1", client: client}
	ctx, cancel := context.WithCancel(context.Background())
	callDone := make(chan error, 1)
	go func() {
		callDone <- event.Actions().Call(ctx, "storage.kv", map[string]any{"operation": "get", "key": "fixture"}, nil)
	}()
	action := <-sink
	cancel()
	if err := <-callDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled caller = %v", err)
	}
	terminalDone := make(chan error, 1)
	go func() { terminalDone <- event.Result(map[string]any{}) }()
	if !client.routeResponse(protocolFrame{Type: "result", RequestID: action.RequestID}) {
		t.Fatal("known late response was rejected")
	}
	if err := <-terminalDone; err != nil {
		t.Fatal(err)
	}
	if frame := <-sink; frame.Type != "result" || frame.RequestID != event.RequestID {
		t.Fatalf("terminal = %#v", frame)
	}
	if err := event.Actions().Call(context.Background(), "storage.kv", nil, nil); err == nil {
		t.Fatal("action accepted after terminal")
	}
	if len(sink) != 0 {
		t.Fatal("extra protocol frames emitted")
	}
}

func TestUnsettledActionsWithholdTerminalAndRecognizeLateResponse(t *testing.T) {
	sink := make(actionFrameSink, 8)
	client := newRuntimeClient(sink, 10*time.Millisecond)
	event := &EventContext{RequestID: "event-1", client: client}
	ctx, cancel := context.WithCancel(context.Background())
	callDone := make(chan error, 1)
	go func() {
		callDone <- event.Actions().Call(ctx, "logger.write", map[string]any{"level": "info", "message": "fixture"}, nil)
	}()
	action := <-sink
	cancel()
	<-callDone
	if err := event.Result(nil); err == nil {
		t.Fatal("unsettled action allowed a terminal frame")
	}
	if len(sink) != 0 {
		t.Fatal("terminal emitted before host action settled")
	}
	if !client.routeResponse(protocolFrame{Type: "result", RequestID: action.RequestID}) {
		t.Fatal("retired action response was rejected")
	}
	if client.routeResponse(protocolFrame{Type: "result", RequestID: "unknown"}) {
		t.Fatal("unknown response was accepted")
	}
}

func TestCanceledActionDoesNotWriteFrame(t *testing.T) {
	sink := make(actionFrameSink, 8)
	event := &EventContext{RequestID: "event-1", client: newRuntimeClient(sink, time.Second)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := event.Actions().Call(ctx, "message.send", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled action = %v", err)
	}
	if len(sink) != 0 {
		t.Fatal("canceled action was sent")
	}
}

func TestTerminalDrainReleasesActionWaiterWithLongerDeadline(t *testing.T) {
	sink := make(actionFrameSink, 8)
	client := newRuntimeClient(sink, 10*time.Millisecond)
	event := &EventContext{RequestID: "event-1", client: client}
	ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
	defer cancel()
	callDone := make(chan error, 1)
	go func() {
		callDone <- event.Actions().Call(ctx, "storage.kv", map[string]any{"operation": "get", "key": "fixture"}, nil)
	}()
	<-sink
	if err := event.Result(nil); err == nil {
		t.Fatal("unsettled terminal accepted")
	}
	select {
	case err := <-callDone:
		var actionErr *ActionError
		if !errors.As(err, &actionErr) || actionErr.Code != "plugin.event_canceled" {
			t.Fatalf("drain result = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("retired action waiter was leaked")
	}
}
