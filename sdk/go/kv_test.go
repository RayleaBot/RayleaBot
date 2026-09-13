package rayleabot

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestKVTTLValidationDoesNotEmitInvalidRequests(t *testing.T) {
	for _, ttl := range []time.Duration{-time.Second, time.Nanosecond, 1500 * time.Millisecond, 31536001 * time.Second} {
		sink := make(actionFrameSink, 1)
		event := &EventContext{RequestID: "fixture", client: newRuntimeClient(sink, time.Second)}
		if _, err := event.Actions().KVSetWithOptions(context.Background(), "k", 1, KVSetOptions{TTL: ttl}); err == nil {
			t.Fatalf("invalid TTL accepted: %s", ttl)
		}
		select {
		case <-sink:
			t.Fatal("invalid TTL emitted a frame")
		default:
		}
	}
}

func TestKVTTLWireRoundTripAndPermanentNull(t *testing.T) {
	for _, ttl := range []time.Duration{0, time.Second, 31536000 * time.Second} {
		sink := make(actionFrameSink, 1)
		client := newRuntimeClient(sink, time.Second)
		event := &EventContext{RequestID: "fixture", client: client}
		done := make(chan error, 1)
		go func() {
			result, err := event.Actions().KVSetWithOptions(context.Background(), "k", nil, KVSetOptions{TTL: ttl})
			if err == nil && (result.ExpiresAtMS != nil) != (ttl != 0) {
				t.Error("expiry presence lost")
			}
			done <- err
		}()
		var frame protocolFrame
		select {
		case frame = <-sink:
		case <-time.After(time.Second):
			t.Fatal("KV request not emitted")
		}
		var input map[string]any
		if err := json.Unmarshal(frame.Data, &input); err != nil {
			t.Fatal(err)
		}
		if value, present := input["value"]; !present || value != nil {
			t.Fatal("explicit null was omitted")
		}
		if ttl == 0 {
			if _, present := input["ttl_seconds"]; present {
				t.Fatal("zero TTL must be omitted")
			}
		} else if input["ttl_seconds"] != ttl.Seconds() {
			t.Fatalf("TTL changed: %#v", input)
		}
		response := json.RawMessage(`{}`)
		if ttl > 0 {
			response = json.RawMessage(`{"expires_at_ms":1789300000000}`)
		}
		client.routeResponse(protocolFrame{Type: "result", RequestID: frame.RequestID, Status: "success", Data: response})
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func TestKVTTLRejectsInvalidHostExpiry(t *testing.T) {
	for _, response := range []json.RawMessage{json.RawMessage(`{"expires_at_ms":"tomorrow"}`), json.RawMessage(`{}`)} {
		sink := make(actionFrameSink, 1)
		client := newRuntimeClient(sink, time.Second)
		event := &EventContext{RequestID: "fixture", client: client}
		done := make(chan error, 1)
		go func() {
			_, err := event.Actions().KVSetWithOptions(context.Background(), "k", 1, KVSetOptions{TTL: time.Second})
			done <- err
		}()
		frame := <-sink
		client.routeResponse(protocolFrame{Type: "result", RequestID: frame.RequestID, Status: "success", Data: response})
		if err := <-done; err == nil {
			t.Fatal("invalid host deadline accepted")
		}
	}
}

func ExampleActions_KVSetWithOptions() {
	_ = HandlerFunc(func(ctx context.Context, event *EventContext) error {
		_, err := event.Actions().KVSetWithOptions(ctx, "draft", map[string]any{"selection": "weather"}, KVSetOptions{TTL: 5 * time.Minute})
		return err
	})
}
