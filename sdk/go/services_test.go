package rayleabot

import (
	"context"
	"encoding/json"
	"io"
	"sync/atomic"
	"testing"
	"time"
)

func TestServiceCancellationRemovesQueuedHandlerWithoutBlockingReader(t *testing.T) {
	in, send := io.Pipe()
	receive, out := io.Pipe()
	t.Cleanup(func() { _ = send.Close(); _ = in.Close(); _ = out.Close(); _ = receive.Close() })
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	done := make(chan error, 1)
	go func() {
		done <- Run(t.Context(), Options{Stdin: in, Stdout: out, Stderr: io.Discard, Services: []Service{{
			Name: "resource", Version: 1, Methods: map[string]ServiceHandler{
				"query": func(context.Context, *EventContext, ServiceRequest) (map[string]any, error) {
					calls.Add(1)
					return map[string]any{}, nil
				},
			},
		}}}, HandlerFunc(func(ctx context.Context, event *EventContext) error {
			close(entered)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	}()
	frames := make(chan protocolFrame, 8)
	go func() {
		decoder := json.NewDecoder(receive)
		for {
			var frame protocolFrame
			if decoder.Decode(&frame) != nil {
				return
			}
			frames <- frame
		}
	}()
	write := func(value any) {
		t.Helper()
		if err := json.NewEncoder(send).Encode(value); err != nil {
			t.Fatal(err)
		}
	}
	read := func(kind, id string) {
		t.Helper()
		select {
		case frame := <-frames:
			if frame.Type != kind || frame.RequestID != id {
				t.Fatalf("unexpected frame: %#v", frame)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for %s/%s", kind, id)
		}
	}
	write(map[string]any{"type": "init", "request_id": "init-1", "protocol_version": "4", "plugin_id": "provider", "timezone": "UTC", "concurrency": 1, "bots": []any{}})
	read("init_ack", "init-1")
	write(map[string]any{"type": "event", "request_id": "ordinary", "event": map[string]any{"event_id": "ordinary", "event_type": "management.action", "source_protocol": "platform", "source_adapter": "management.internal", "timestamp": 1}})
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("ordinary handler did not start")
	}
	write(map[string]any{"type": "event", "request_id": "queued", "event": map[string]any{
		"event_id": "queued", "event_type": "plugin.request", "source_protocol": "platform", "source_adapter": "plugins.internal", "timestamp": 1,
		"payload": map[string]any{"service_request": ServiceRequest{CallerPluginID: "consumer", Service: "resource", ServiceVersion: 1, Method: "query", Params: map[string]any{}, DeadlineAtMs: time.Now().Add(time.Minute).UnixMilli()}},
	}})
	write(map[string]any{"type": "cancel", "request_id": "queued"})
	write(map[string]any{"type": "ping", "request_id": "barrier"})
	read("pong", "barrier")
	close(release)
	read("result", "ordinary")
	write(map[string]any{"type": "shutdown", "request_id": "stop", "reason": "test"})
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runtime did not settle handlers")
	}
	if calls.Load() != 0 {
		t.Fatal("canceled queued service handler ran")
	}
}

func TestServiceRegistrationsAreValidatedAndCopied(t *testing.T) {
	handler := ServiceHandler(func(context.Context, *EventContext, ServiceRequest) (map[string]any, error) { return nil, nil })
	service := Service{Name: "resource", Version: 1, Methods: map[string]ServiceHandler{"query": handler}}
	handlers, err := compileServices([]Service{service})
	if err != nil {
		t.Fatal(err)
	}
	delete(service.Methods, "query")
	if handlers[serviceMethod{name: "resource", version: 1, method: "query"}] == nil {
		t.Fatal("registration changed after caller mutated its map")
	}
	for _, services := range [][]Service{
		{{Name: "resource", Version: 1, Methods: map[string]ServiceHandler{"query": nil}}},
		{{Name: "bad name", Version: 1, Methods: map[string]ServiceHandler{"query": handler}}},
		{{Name: "resource", Version: 0, Methods: map[string]ServiceHandler{"query": handler}}},
		{{Name: "resource", Version: 1, Methods: map[string]ServiceHandler{"query": handler}}, {Name: "resource", Version: 1, Methods: map[string]ServiceHandler{"other": handler}}},
	} {
		if _, err := compileServices(services); err == nil {
			t.Fatal("invalid service registration was accepted")
		}
	}
}
