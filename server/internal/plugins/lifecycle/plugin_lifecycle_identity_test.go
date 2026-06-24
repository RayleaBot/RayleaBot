package lifecycle

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func TestBroadcastBotIdentityChangedDispatchesToRunningPlugin(t *testing.T) {
	t.Parallel()

	dispatcher := dispatch.New(slog.Default(), nil, nil, 16)
	fakeRuntime := &capturingRuntime{events: make(chan runtimeprotocol.Event, 1)}
	dispatcher.Register("weather", fakeRuntime, []string{"message.group"}, nil, 1)

	controller := NewController(Deps{
		CurrentConfig: newTestAppState(config.Config{}, nil).state.CurrentConfig,
		Logger:        slog.Default(),
		Dispatcher:    dispatcher,
	})

	controller.broadcastBotIdentityChanged(context.Background(), "10001")

	select {
	case event := <-fakeRuntime.events:
		if event.EventType != "bot.identity.changed" {
			t.Fatalf("event_type = %q, want bot.identity.changed", event.EventType)
		}
		if event.Target == nil || event.Target.Type != "bot" || event.Target.ID != "10001" {
			t.Fatalf("unexpected identity target: %#v", event.Target)
		}
		onebot, ok := event.PayloadFields["onebot"].(map[string]any)
		if !ok || onebot["self_id"] != "10001" {
			t.Fatalf("unexpected onebot identity payload: %#v", event.PayloadFields)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected bot.identity.changed event")
	}
}
