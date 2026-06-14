package bridge

import (
	"strings"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
	adaptersegments "github.com/RayleaBot/RayleaBot/server/internal/adapter/segments"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/runtime/protocol"
)

func runtimeEventFromAdapter(event adapterintake.NormalizedEvent) runtimeprotocol.Event {
	runtimeEvent := runtimeprotocol.Event{
		EventID:        event.EventID,
		SourceProtocol: event.SourceProtocol,
		SourceAdapter:  event.SourceAdapter,
		EventType:      event.EventType,
		Timestamp:      event.Timestamp,
		Actor: &runtimeprotocol.EventActor{
			ID:       event.SenderID,
			Nickname: event.ActorNickname,
			Role:     event.ActorRole,
		},
		Target: &runtimeprotocol.EventTarget{
			Type: bridgeTargetType(event),
			ID:   bridgeTargetID(event),
			Name: event.TargetName,
		},
		PayloadFields: event.PayloadFields,
		MessageID:     event.MessageID,
	}
	if event.PlainText != "" || len(event.Segments) > 0 {
		runtimeEvent.Message = &runtimeprotocol.EventMessage{
			PlainText: event.PlainText,
			Segments:  runtimeSegmentsFromAdapter(event.Segments),
		}
	}
	return runtimeEvent
}

func runtimeSegmentsFromAdapter(segments []adaptersegments.MessageSegment) []runtimeprotocol.EventSegment {
	if len(segments) == 0 {
		return nil
	}
	projected := make([]runtimeprotocol.EventSegment, 0, len(segments))
	for _, seg := range segments {
		projected = append(projected, runtimeprotocol.EventSegment{
			Type: seg.Type,
			Data: seg.Data,
		})
	}
	return projected
}

func bridgeTargetType(event adapterintake.NormalizedEvent) string {
	if strings.TrimSpace(event.TargetType) != "" {
		return event.TargetType
	}
	return event.ConversationType
}

func bridgeTargetID(event adapterintake.NormalizedEvent) string {
	if strings.TrimSpace(event.TargetID) != "" {
		return event.TargetID
	}
	return event.ConversationID
}
