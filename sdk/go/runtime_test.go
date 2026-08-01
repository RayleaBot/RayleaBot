package rayleabot

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunCorrelatesConcurrentLocalActionsAndSerializesTerminalFrames(t *testing.T) {
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	defer inputReader.Close()
	defer outputReader.Close()

	results := make(chan string, 2)
	handler := HandlerFunc(func(ctx context.Context, event *EventContext) error {
		result, err := event.Actions().KVGet(ctx, event.Event.EventID)
		if err != nil {
			return err
		}
		value, _ := result["value"].(string)
		results <- event.Event.EventID + ":" + value
		return event.Result(map[string]any{"value": value})
	})

	runDone := make(chan error, 1)
	go func() {
		runDone <- Run(context.Background(), Options{
			PluginID:              "test-plugin",
			Stdin:                 inputReader,
			Stdout:                outputWriter,
			ActionTimeout:         time.Second,
			MaxConcurrentHandlers: 2,
		}, handler)
	}()

	encoder := json.NewEncoder(inputWriter)
	decoder := json.NewDecoder(outputReader)
	writeFrame(t, encoder, protocolFrame{ProtocolVersion: "1", Type: "init", PluginID: "test-plugin", RequestID: "init-1"})
	var initAck protocolFrame
	decodeFrame(t, decoder, &initAck)
	if initAck.Type != "init_ack" || initAck.Status != "ready" {
		t.Fatalf("unexpected init response: %#v", initAck)
	}
	writeEvent(t, encoder, "event-1", "alpha")
	writeEvent(t, encoder, "event-2", "beta")

	var actions [2]protocolFrame
	decodeFrame(t, decoder, &actions[0])
	decodeFrame(t, decoder, &actions[1])
	if actions[0].Type != "action" || actions[1].Type != "action" || actions[0].RequestID == actions[1].RequestID {
		t.Fatalf("unexpected action frames: %#v %#v", actions[0], actions[1])
	}
	for index := len(actions) - 1; index >= 0; index-- {
		value := "for-" + actions[index].ParentRequestID
		data, _ := json.Marshal(map[string]any{"value": value})
		writeFrame(t, encoder, protocolFrame{ProtocolVersion: "1", Type: "result", PluginID: "test-plugin", RequestID: actions[index].RequestID, Data: data})
	}

	var terminal [2]protocolFrame
	decodeFrame(t, decoder, &terminal[0])
	decodeFrame(t, decoder, &terminal[1])
	for _, frame := range terminal {
		if frame.Type != "result" || (frame.RequestID != "event-1" && frame.RequestID != "event-2") {
			t.Fatalf("unexpected terminal frame: %#v", frame)
		}
	}

	seen := map[string]bool{}
	seen[<-results] = true
	seen[<-results] = true
	if !seen["alpha:for-event-1"] || !seen["beta:for-event-2"] {
		t.Fatalf("local actions were mis-correlated: %#v", seen)
	}
	writeFrame(t, encoder, protocolFrame{ProtocolVersion: "1", Type: "shutdown", PluginID: "test-plugin", RequestID: "shutdown-1"})
	if err := <-runDone; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunEnforcesOneTerminalResponseAndIsolatesPanics(t *testing.T) {
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	defer inputReader.Close()
	defer outputReader.Close()

	var secondError error
	var mu sync.Mutex
	secondDone := make(chan struct{})
	handler := HandlerFunc(func(_ context.Context, event *EventContext) error {
		if event.Event.EventID == "panic" {
			panic("token=fixture-secret")
		}
		if err := event.Result(map[string]any{"ok": true}); err != nil {
			return err
		}
		mu.Lock()
		secondError = event.Fail("plugin.internal_error", "should not be emitted")
		mu.Unlock()
		close(secondDone)
		return nil
	})
	runDone := make(chan error, 1)
	go func() {
		runDone <- Run(context.Background(), Options{PluginID: "test-plugin", Stdin: inputReader, Stdout: outputWriter}, handler)
	}()
	encoder := json.NewEncoder(inputWriter)
	decoder := json.NewDecoder(outputReader)
	writeFrame(t, encoder, protocolFrame{ProtocolVersion: "1", Type: "init", PluginID: "test-plugin", RequestID: "init"})
	var frame protocolFrame
	decodeFrame(t, decoder, &frame)
	writeEvent(t, encoder, "first", "first")
	decodeFrame(t, decoder, &frame)
	if frame.Type != "result" {
		t.Fatalf("first terminal type = %q", frame.Type)
	}
	<-secondDone
	mu.Lock()
	err := secondError
	mu.Unlock()
	if err == nil || !strings.Contains(err.Error(), "already sent") {
		t.Fatalf("second terminal response error = %v", err)
	}

	writeEvent(t, encoder, "panic-request", "panic")
	decodeFrame(t, decoder, &frame)
	if frame.Type != "error" || frame.Code != "plugin.internal_error" || strings.Contains(frame.Message, "fixture-secret") {
		t.Fatalf("panic response leaked details or used wrong code: %#v", frame)
	}
	writeFrame(t, encoder, protocolFrame{ProtocolVersion: "1", Type: "shutdown", PluginID: "test-plugin", RequestID: "shutdown"})
	if err := <-runDone; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func writeEvent(t *testing.T, encoder *json.Encoder, requestID, eventID string) {
	t.Helper()
	payload, _ := json.Marshal(Event{EventID: eventID, EventType: "message", Target: Target{Type: "group", ID: "100"}})
	writeFrame(t, encoder, protocolFrame{ProtocolVersion: "1", Type: "event", PluginID: "test-plugin", RequestID: requestID, Event: payload})
}

func writeFrame(t *testing.T, encoder *json.Encoder, frame protocolFrame) {
	t.Helper()
	if err := encoder.Encode(frame); err != nil {
		t.Fatalf("encode frame: %v", err)
	}
}

func decodeFrame(t *testing.T, decoder *json.Decoder, frame *protocolFrame) {
	t.Helper()
	if err := decoder.Decode(frame); err != nil {
		t.Fatalf("decode frame: %v", err)
	}
}
