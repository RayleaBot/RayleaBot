package rayleabot

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type actionFrameSink chan protocolFrame

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
	go func() { callDone <- event.Actions().Call(ctx, "storage.kv", map[string]any{}, nil) }()
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
	go func() { callDone <- event.Actions().Call(ctx, "message.send", map[string]any{}, nil) }()
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
	go func() { callDone <- event.Actions().Call(ctx, "storage.kv", nil, nil) }()
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
