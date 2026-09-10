package testutil

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

type EventRuntime struct{ Events chan chatevent.Event }

func (r *EventRuntime) DeliverEvent(ctx context.Context, event chatevent.Event) (plugins.Delivery, error) {
	select {
	case r.Events <- event:
		return plugins.Delivery{RequestID: "event_fixture", Result: map[string]any{}}, nil
	case <-ctx.Done():
		return plugins.Delivery{}, ctx.Err()
	}
}
func (*EventRuntime) Snapshot() pluginruntime.Snapshot {
	return pluginruntime.Snapshot{State: pluginruntime.StateRunning}
}
func (*EventRuntime) ReadyForEvents() bool { return true }
