package bridge

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func runtimeEventFromAdapter(event adapter.NormalizedEvent) runtime.Event {
	runtimeEvent := runtime.Event{
		EventID:        event.EventID,
		SourceProtocol: event.SourceProtocol,
		SourceAdapter:  event.SourceAdapter,
		EventType:      event.EventType,
		Timestamp:      event.Timestamp,
		Actor: &runtime.EventActor{
			ID:       event.SenderID,
			Nickname: event.ActorNickname,
			Role:     event.ActorRole,
		},
		Target: &runtime.EventTarget{
			Type: bridgeTargetType(event),
			ID:   bridgeTargetID(event),
			Name: event.TargetName,
		},
		PayloadFields: event.PayloadFields,
		MessageID:     event.MessageID,
	}
	if event.PlainText != "" || len(event.Segments) > 0 {
		runtimeEvent.Message = &runtime.EventMessage{
			PlainText: event.PlainText,
			Segments:  runtimeSegmentsFromAdapter(event.Segments),
		}
	}
	return runtimeEvent
}

func runtimeSegmentsFromAdapter(segments []adapter.MessageSegment) []runtime.EventSegment {
	if len(segments) == 0 {
		return nil
	}
	projected := make([]runtime.EventSegment, 0, len(segments))
	for _, seg := range segments {
		projected = append(projected, runtime.EventSegment{
			Type: seg.Type,
			Data: seg.Data,
		})
	}
	return projected
}

func bridgeTargetType(event adapter.NormalizedEvent) string {
	if strings.TrimSpace(event.TargetType) != "" {
		return event.TargetType
	}
	return event.ConversationType
}

func bridgeTargetID(event adapter.NormalizedEvent) string {
	if strings.TrimSpace(event.TargetID) != "" {
		return event.TargetID
	}
	return event.ConversationID
}
