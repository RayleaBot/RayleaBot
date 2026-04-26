package app

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func TestBroadcastBotIdentityChangedDispatchesToRunningPlugin(t *testing.T) {
	t.Parallel()

	dispatcher := dispatch.New(slog.Default(), nil, nil, 16)
	fakeRuntime := &capturingRuntime{events: make(chan runtime.Event, 1)}
	dispatcher.Register("weather", fakeRuntime, []string{"message.group"}, nil, 1)

	controller := newPluginLifecycleController(pluginLifecycleDeps{
		state:      newTestAppState(config.Config{}, nil).state,
		dispatcher: dispatcher,
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
