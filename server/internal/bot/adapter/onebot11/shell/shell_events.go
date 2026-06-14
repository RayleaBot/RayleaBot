package shell

import (
	"context"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
)

func (s *Shell) forwardSupportedEvent(ctx context.Context, transport TransportKey, frame adapterintake.ClassifiedFrame) {
	if frame.Summary.Category != adapterintake.FrameCategoryEvent {
		return
	}

	s.invalidateIdentityCacheForFrame(frame.Frame)

	normalizedEvent, ok := normalizeSupportedEvent(frame.Frame, frame.Summary.ObservedAt)
	if !ok {
		s.logger.Debug(
			"adapter event ignored by runtime bridge",
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"transport", string(transport),
			"frame_type", frame.Summary.Type,
		)
		return
	}
	if s.isDuplicateEvent(normalizedEvent.EventID, frame.Summary.ObservedAt) {
		s.logger.Info(
			"duplicate OneBot event dropped",
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"transport", string(transport),
			"error_code", errorCodeWebhookDuplicateEvent,
			"event_id", normalizedEvent.EventID,
			"event_type", normalizedEvent.EventType,
		)
		return
	}

	handler := s.currentEventHandler()
	if handler == nil {
		return
	}

	select {
	case s.eventQueue <- normalizedEvent:
	case <-ctx.Done():
		return
	default:
		s.logger.Warn(
			"adapter event queue is full; dropping event",
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"event_kind", normalizedEvent.Kind,
			"event_type", normalizedEvent.EventType,
		)
	}
}
func (s *Shell) currentEventHandler() func(context.Context, adapterintake.NormalizedEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.eventHandler
}
func (s *Shell) currentReadyHandler() func(context.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readyHandler
}
func (s *Shell) dispatchEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-s.eventQueue:
			handler := s.currentEventHandler()
			if handler == nil {
				continue
			}
			handler(ctx, event)
		}
	}
}
