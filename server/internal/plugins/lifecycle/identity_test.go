package lifecycle

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

type identitySource struct{ bots []chatevent.BotIdentity }

func (s *identitySource) BotIdentities() []chatevent.BotIdentity {
	return append([]chatevent.BotIdentity{}, s.bots...)
}

func TestIdentitySnapshotsPreserveNamespacesAndClearRemovedInstances(t *testing.T) {
	t.Parallel()
	dispatcher := dispatch.New(slog.Default(), nil, nil, 16, 4)
	t.Cleanup(dispatcher.Close)
	capture := &capturingRuntime{events: make(chan pluginruntime.Event, 4)}
	dispatcher.Register("fixture", capture, nil, nil, 1)
	source := &identitySource{bots: []chatevent.BotIdentity{
		{SourceAdapter: "onebot", SourceProtocol: "onebot11", ID: "shared"},
		{SourceAdapter: "qq", SourceProtocol: "qqofficial", ID: "shared"},
	}}
	controller := NewController(Deps{Dispatcher: dispatcher, Identities: source})
	receive := func(want int) {
		t.Helper()
		select {
		case event := <-capture.events:
			bots, ok := event.PayloadFields["bots"].([]chatevent.BotIdentity)
			if !ok || len(bots) != want || event.SourceProtocol != "platform" || event.SourceAdapter != "adapters.internal" {
				t.Fatalf("identity event=%#v", event)
			}
			if want == 2 && (bots[0].SourceAdapter == bots[1].SourceAdapter || bots[0].SourceProtocol == bots[1].SourceProtocol) {
				t.Fatalf("namespaces lost: %#v", bots)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("identity snapshot not delivered")
		}
	}
	controller.SyncBotIdentities(context.Background())
	receive(2)
	before := dispatcher.Stats().Delivered
	controller.SyncBotIdentities(context.Background())
	if dispatcher.Stats().Delivered != before {
		t.Fatal("unchanged snapshot dispatched twice")
	}
	source.bots = source.bots[:1]
	controller.SyncBotIdentities(context.Background())
	receive(1)
	source.bots = nil
	controller.SyncBotIdentities(context.Background())
	receive(0)
}

func TestAfterRuntimeRegisteredDispatchesPluginStarted(t *testing.T) {
	t.Parallel()

	dispatcher := dispatch.New(slog.Default(), nil, nil, 16)
	t.Cleanup(dispatcher.Close)
	fakeRuntime := &capturingRuntime{events: make(chan pluginruntime.Event, 1)}
	dispatcher.Register("raylea.subscription-hub", fakeRuntime, nil, nil, 1)

	controller := NewController(Deps{
		CurrentConfig: newTestAppState(config.Config{}, nil).state.CurrentConfig,
		Logger:        slog.Default(),
		Dispatcher:    dispatcher,
	})

	controller.afterRuntimeRegistered(context.Background(), "raylea.subscription-hub", nil)

	select {
	case event := <-fakeRuntime.events:
		if event.EventType != "plugin.started" {
			t.Fatalf("event_type = %q, want plugin.started", event.EventType)
		}
		if event.SourceProtocol != "platform" || event.SourceAdapter != "plugin.lifecycle" {
			t.Fatalf("unexpected started source: %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected plugin.started event")
	}
}
