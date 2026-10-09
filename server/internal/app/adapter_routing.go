package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
)

func (s EventState) ResolveOneBotAdapter(sourceAdapter, sourceProtocol string) (actions.OneBotAdapter, error) {
	if s.Adapters == nil {
		return nil, fmt.Errorf("OneBot adapter registry is unavailable")
	}
	snapshot := s.Adapters.Snapshot()
	routes := adapterRoutes(snapshot)
	router := outbound.NewRouter(routes.Senders, routes.Protocols, func() config.Config { return *routes.Config })
	id, err := router.ResolveAdapterID(sourceAdapter, sourceProtocol)
	if err != nil {
		return nil, err
	}
	shell := snapshot.OneBot11(id)
	if shell == nil {
		return nil, fmt.Errorf("adapter %q does not serve OneBot11 actions", id)
	}
	return observedOneBotAdapter{OneBotAdapter: shell, id: id, messageSent: s.MessageSent}, nil
}

// Forward sends bypass the outbound Router. Observe the resolved instance,
// counting the whole forward once, irrespective of its number of nodes.
type observedOneBotAdapter struct {
	actions.OneBotAdapter
	id          string
	messageSent func(string, string)
}

func (a observedOneBotAdapter) CallAPIAny(ctx context.Context, action string, params map[string]any) (any, error) {
	result, err := a.OneBotAdapter.CallAPIAny(ctx, action, params)
	if err == nil && (action == "send_group_forward_msg" || action == "send_private_forward_msg") {
		a.messageSent(a.id, "onebot11")
	}
	return result, err
}

func (s EventState) EnrichEventMetadata(ctx context.Context, event chatevent.NormalizedEvent) chatevent.NormalizedEvent {
	if event.SourceProtocol != config.AdapterTypeOneBot11 || strings.TrimSpace(event.SourceAdapter) == "" || s.AdapterRouter == nil {
		return event
	}
	snapshot := s.Adapters.Snapshot()
	routes := adapterRoutes(snapshot)
	router := outbound.NewRouter(routes.Senders, routes.Protocols, func() config.Config { return *routes.Config })
	id, err := router.ResolveAdapterID(event.SourceAdapter, event.SourceProtocol)
	if err != nil {
		return event
	}
	if shell := snapshot.OneBot11(id); shell != nil {
		return shell.EnrichEventMetadata(ctx, event)
	}
	return event
}
