package app

import (
	"context"
	"log/slog"
	"testing"
	"time"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

type metadataEnricherStub struct {
	calls int
}

func (s *metadataEnricherStub) EnrichEventMetadata(_ context.Context, event adapterintake.NormalizedEvent) adapterintake.NormalizedEvent {
	s.calls++
	event.TargetName = "测试群"
	return event
}

type eventIngressDispatcherStub struct {
	events []runtimeprotocol.Event
}

func (*eventIngressDispatcherStub) HasDeliverablePlugins() bool {
	return true
}

func (s *eventIngressDispatcherStub) Dispatch(_ context.Context, event runtimeprotocol.Event, _ string) []dispatch.DeliveryResult {
	s.events = append(s.events, event)
	return []dispatch.DeliveryResult{{
		PluginID: "echo",
		Outcome:  dispatch.OutcomeDelivered,
	}}
}

func TestEventIngressEnrichesMetadataBeforeBridgeDispatch(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{}, nil)
	dispatcher := &eventIngressDispatcherStub{}
	application.setTestEventIngress(nil, nil, nil, bridge.New(slog.Default(), dispatcher))
	enricher := &metadataEnricherStub{}
	application.services.eventIngress.SetMetadataEnricher(enricher)

	application.handleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "onebot11-message-1001",
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Unix(1_700_000_123, 0).Unix(),
		ConversationType: "group",
		ConversationID:   "2001",
		SenderID:         "3001",
		PlainText:        "hello bridge",
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"post_type":    "message",
				"message_type": "group",
				"group_id":     "2001",
				"user_id":      "3001",
				"sender": map[string]any{
					"nickname": "测试用户A",
				},
			},
		},
	})

	if enricher.calls != 1 {
		t.Fatalf("expected metadata enricher to be called once, got %d", enricher.calls)
	}
	if len(dispatcher.events) != 1 {
		t.Fatalf("expected one dispatched event, got %d", len(dispatcher.events))
	}
	if dispatcher.events[0].Target == nil || dispatcher.events[0].Target.Name != "测试群" {
		t.Fatalf("unexpected dispatched target: %#v", dispatcher.events[0].Target)
	}
}
