package bridge

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func runtimeEventFromAdapter(event onebot11.NormalizedEvent) pluginruntime.Event {
	runtimeEvent := pluginruntime.Event{
		EventID:        event.EventID,
		SourceProtocol: event.SourceProtocol,
		SourceAdapter:  event.SourceAdapter,
		EventType:      event.EventType,
		Timestamp:      event.Timestamp,
		Actor: &pluginruntime.EventActor{
			ID:       event.SenderID,
			Nickname: event.ActorNickname,
			Role:     event.ActorRole,
		},
		Target: &pluginruntime.EventTarget{
			Type: bridgeTargetType(event),
			ID:   bridgeTargetID(event),
			Name: event.TargetName,
		},
		PayloadFields: event.PayloadFields,
		MessageID:     event.MessageID,
	}
	if event.PlainText != "" || len(event.Segments) > 0 {
		runtimeEvent.Message = &pluginruntime.EventMessage{
			PlainText: event.PlainText,
			Segments:  runtimeSegmentsFromAdapter(event.Segments),
		}
	}
	return runtimeEvent
}

func runtimeSegmentsFromAdapter(segments []onebot11.MessageSegment) []pluginruntime.EventSegment {
	if len(segments) == 0 {
		return nil
	}
	projected := make([]pluginruntime.EventSegment, 0, len(segments))
	for _, seg := range segments {
		projected = append(projected, pluginruntime.EventSegment{
			Type: seg.Type,
			Data: seg.Data,
		})
	}
	return projected
}

func bridgeTargetType(event onebot11.NormalizedEvent) string {
	if strings.TrimSpace(event.TargetType) != "" {
		return event.TargetType
	}
	return event.ConversationType
}

func bridgeTargetID(event onebot11.NormalizedEvent) string {
	if strings.TrimSpace(event.TargetID) != "" {
		return event.TargetID
	}
	return event.ConversationID
}
