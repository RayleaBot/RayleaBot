package app

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func inboundRoutingEvent(t *testing.T, id string) chatevent.NormalizedEvent {
	t.Helper()
	shell := onebot11.New(id, config.OneBotConfig{}, config.AdapterConfig{}, discardLogger())
	events := make(chan chatevent.NormalizedEvent, 1)
	shell.SetEventHandler(func(_ context.Context, event chatevent.NormalizedEvent) { events <- event })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		if err := shell.Stop(stopCtx); err != nil {
			t.Error(err)
		}
	})
	shell.Start(ctx)
	if err := shell.AcceptWebhookPayload(ctx, []byte(`{"post_type":"message","message_type":"group","self_id":101,"user_id":201,"group_id":301,"message_id":42,"raw_message":"fixture"}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-events:
		if event.SourceAdapter != id {
			t.Fatalf("source_adapter = %q, want %q", event.SourceAdapter, id)
		}
		return event
	case <-time.After(time.Second):
		t.Fatal("event was not delivered")
		return chatevent.NormalizedEvent{}
	}
}

func TestReplyRoutingIsolatesEqualUpstreamMessageIDs(t *testing.T) {
	first := inboundRoutingEvent(t, "first")
	second := inboundRoutingEvent(t, "second")
	if first.EventID == second.EventID {
		t.Fatal("two adapters emitted the same host event ID")
	}
	cache := outbound.NewReplyTargetCache(100)
	cache.Record(first)
	cache.Record(second)
	var sent []string
	router := newAdapterRouter(map[string]outbound.ActionSender{
		"first":  recordingSender{name: "first", sent: &sent},
		"second": recordingSender{name: "second", sent: &sent},
	}, map[string]string{"first": "onebot11", "second": "onebot11"})
	for _, event := range []chatevent.NormalizedEvent{first, second} {
		if _, err := outbound.SendAction(context.Background(), router, cache, pluginruntime.EventFromAdapter(event), pluginruntime.Action{Kind: "message.reply", ReplyToEventID: event.EventID}); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(sent, []string{"first-reply", "second-reply"}) {
		t.Fatalf("sent via %v", sent)
	}
}

func TestActiveSendSelectsInstanceAndTracksEnableSwitch(t *testing.T) {
	var sent []string
	cfg := config.Config{Adapters: []config.AdapterInstance{
		{ID: "first", Type: "onebot11", Enabled: true},
		{ID: "second", Type: "onebot11", Enabled: true},
	}}
	router := newAdapterRouter(map[string]outbound.ActionSender{
		"first":  recordingSender{name: "first", sent: &sent},
		"second": recordingSender{name: "second", sent: &sent},
	}, map[string]string{"first": "onebot11", "second": "onebot11"})
	router.currentConfig = func() config.Config { return cfg }
	action := pluginruntime.Action{Kind: "message.send", SourceAdapter: "second", SourceProtocol: "onebot11", TargetType: "group", TargetID: "301"}
	if _, err := outbound.SendAction(context.Background(), router, nil, pluginruntime.Event{}, action); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[1].Enabled = false
	if _, err := outbound.SendAction(context.Background(), router, nil, pluginruntime.Event{}, action); err == nil {
		t.Fatal("disabled instance accepted a send")
	}
	action.SourceAdapter = ""
	if _, err := outbound.SendAction(context.Background(), router, nil, pluginruntime.Event{}, action); err != nil {
		t.Fatal(err)
	}
	cfg.Adapters[1].Enabled = true
	action.SourceAdapter = "second"
	action.SourceProtocol = "qqofficial"
	if _, err := outbound.SendAction(context.Background(), router, nil, pluginruntime.Event{}, action); err == nil {
		t.Fatal("protocol mismatch accepted")
	}
	if !reflect.DeepEqual(sent, []string{"second", "first"}) {
		t.Fatalf("sent via %v", sent)
	}
}
