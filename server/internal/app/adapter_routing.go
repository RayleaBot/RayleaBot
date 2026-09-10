package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
)

func (s EventState) ResolveOneBotAdapter(sourceAdapter, sourceProtocol string) (actions.OneBotAdapter, error) {
	if s.AdapterRouter == nil {
		return nil, fmt.Errorf("OneBot adapter registry is unavailable")
	}
	id, err := s.AdapterRouter.ResolveAdapterID(sourceAdapter, sourceProtocol)
	if err != nil {
		return nil, err
	}
	shell := s.OneBotShells[id]
	if shell == nil {
		return nil, fmt.Errorf("adapter %q does not serve OneBot11 actions", id)
	}
	return shell, nil
}

func (s EventState) EnrichEventMetadata(ctx context.Context, event chatevent.NormalizedEvent) chatevent.NormalizedEvent {
	if event.SourceProtocol != config.AdapterTypeOneBot11 || strings.TrimSpace(event.SourceAdapter) == "" || s.AdapterRouter == nil {
		return event
	}
	id, err := s.AdapterRouter.ResolveAdapterID(event.SourceAdapter, event.SourceProtocol)
	if err != nil {
		return event
	}
	if shell := s.OneBotShells[id]; shell != nil {
		return shell.EnrichEventMetadata(ctx, event)
	}
	return event
}

func (s EventState) DedupDropsSnapshot() uint64 {
	var total uint64
	for _, shell := range s.OneBotShells {
		total += shell.DedupDropsSnapshot()
	}
	return total
}
