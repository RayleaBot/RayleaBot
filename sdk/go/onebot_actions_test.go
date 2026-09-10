package rayleabot

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestOneBotAdapterViewRoutesTypedAndProviderActionsIndependently(t *testing.T) {
	t.Parallel()
	sink := make(actionFrameSink, 4)
	client := newRuntimeClient(sink, time.Second)
	event := &EventContext{RequestID: "scheduler-event", client: client}
	base := event.Actions()
	second := base.ForOneBotAdapter("second")
	for _, tc := range []struct {
		call    func(context.Context) (ActionResult, error)
		adapter string
	}{
		{call: func(ctx context.Context) (ActionResult, error) { return second.GroupInfoGet(ctx, "200") }, adapter: "second"},
		{call: func(ctx context.Context) (ActionResult, error) { return second.NapCatGroupSignSet(ctx, "200") }, adapter: "second"},
		{call: func(ctx context.Context) (ActionResult, error) { return base.GroupInfoGet(ctx, "200") }},
	} {
		done := make(chan error, 1)
		go func() { _, err := tc.call(t.Context()); done <- err }()
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
		if tc.adapter == "" {
			if _, exists := data["source_adapter"]; exists {
				t.Fatal("adapter view mutated original actions")
			}
		} else if data["source_adapter"] != tc.adapter || data["source_protocol"] != "onebot11" {
			t.Fatalf("selector = %#v", data)
		}
		if frame.ParentRequestID != "scheduler-event" || data["group_id"] != "200" {
			t.Fatalf("action context lost: %#v, %#v", frame, data)
		}
		if !client.routeResponse(protocolFrame{Type: "result", RequestID: frame.RequestID}) {
			t.Fatal("response not associated")
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}
