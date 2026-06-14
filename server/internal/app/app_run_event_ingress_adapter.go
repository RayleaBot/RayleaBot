package app

import (
	"context"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
)

func (s *eventIngressService) HandleAdapterEvent(ctx context.Context, event adapterintake.NormalizedEvent) {
	if s == nil {
		return
	}
	event = s.enrichEventMetadata(ctx, event)
	if s.replyTargets != nil {
		s.replyTargets.Record(event)
	}

	enriched, allowed := s.applyChatPolicy(ctx, event)
	if !allowed {
		return
	}

	if s.menu != nil && s.menu.Handle(ctx, enriched) {
		return
	}

	if s.lifecycle != nil {
		s.lifecycle.HandleAdapterEvent(ctx, event)
	}

	if s.bridge != nil {
		s.bridge.HandleAdapterEvent(ctx, enriched)
	}
}

func (s *eventIngressService) enrichEventMetadata(ctx context.Context, event adapterintake.NormalizedEvent) adapterintake.NormalizedEvent {
	if s == nil || s.metadataEnricher == nil {
		return event
	}
	return s.metadataEnricher.EnrichEventMetadata(ctx, event)
}

func (s *eventIngressService) HandleAdapterReady(ctx context.Context) {
	if s == nil || s.lifecycle == nil {
		return
	}

	s.lifecycle.HandleAdapterReady(ctx)
}
