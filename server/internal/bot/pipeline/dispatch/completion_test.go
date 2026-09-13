package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func waitCompletion(t *testing.T, result DeliveryResult) CompletionResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if result.Completion == nil {
		t.Fatal("delivery has no completion")
	}
	completed, err := result.Completion.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := result.Completion.Wait(ctx)
	if err != nil || second != completed {
		t.Fatal("completion is not a stable broadcast result")
	}
	return completed
}

func TestCompletionCoversRejectionAndDraining(t *testing.T) {
	d := New(nil, nil, nil, 1)
	t.Cleanup(d.Close)
	rt := &fakeDeliverer{started: make(chan chatevent.Event, 2), blockCh: make(chan struct{})}
	d.Register("p", rt, nil, nil, 1)
	first := d.DispatchToPlugin(t.Context(), "p", testEvent())
	waitForStartedEvent(t, rt.started)
	second := d.DispatchToPlugin(t.Context(), "p", testEvent())
	full := d.DispatchToPlugin(t.Context(), "p", testEvent())
	if got := waitCompletion(t, full); got.Success || got.ErrorCode != errorcodes.PlatformRateLimited {
		t.Fatalf("full completion=%#v", got)
	}
	drain := d.DrainPlugin("p")
	if got := waitCompletion(t, d.DispatchToPlugin(t.Context(), "p", testEvent())); got.ErrorCode != errorcodes.PluginStopping {
		t.Fatal("stopped admission not settled")
	}
	close(rt.blockCh)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := drain.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if !waitCompletion(t, first).Success || !waitCompletion(t, second).Success {
		t.Fatal("drained delivery did not settle")
	}
	if got := waitCompletion(t, d.DispatchToPlugin(t.Context(), "missing", testEvent())); got.Success {
		t.Fatal("missing target succeeded")
	}
}

func TestCompletionSettlesQueueOnOwnerStop(t *testing.T) {
	d := New(nil, nil, nil, 2)
	rt := &fakeDeliverer{started: make(chan chatevent.Event, 1), blockCh: make(chan struct{})}
	d.Register("p", rt, nil, nil, 1)
	d.DispatchToPlugin(t.Context(), "p", testEvent())
	waitForStartedEvent(t, rt.started)
	queued := d.DispatchToPlugin(t.Context(), "p", testEvent())
	drain := d.DrainPlugin("p")
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := drain.Wait(canceled); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(rt.blockCh)
	d.Close()
	if got := waitCompletion(t, queued); got.Success || got.ErrorCode != errorcodes.PluginEventCanceled {
		t.Fatalf("queued owner stop=%#v", got)
	}
}

func TestCompletionPreservesTerminalPropagationAfterSendFailure(t *testing.T) {
	sender := &fakeSender{sendErr: errors.New("fixture send failed")}
	d := New(nil, sender, nil, 1)
	t.Cleanup(d.Close)
	d.SetPermissionChecker(func(context.Context, string, string) bool { return true })
	rt := &fakeDeliverer{delivery: plugins.Delivery{RequestID: "r", Propagation: "stop", Action: &chatevent.MessageCommand{Kind: "message.send", TargetType: "group", TargetID: "200", MessageSegments: []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "fixture"}}}}}}
	d.Register("p", rt, nil, nil, 1)
	got := waitCompletion(t, d.DispatchToPlugin(t.Context(), "p", testEvent()))
	if !got.Success || got.Propagation != "stop" || len(sender.messages) != 1 {
		t.Fatalf("send changed propagation or completion preceded send: %#v", got)
	}
}
