package chatevent

import (
	"strings"
)

// FromAdapter projects a normalized adapter event onto the plugin runtime
// event shape. It is the single conversion used by every delivery path, so the
// bridge and the builtin menu cannot drift apart on target resolution.
func FromAdapter(event NormalizedEvent) Event {
	runtimeEvent := Event{
		EventID:        event.EventID,
		SourceProtocol: event.SourceProtocol,
		SourceAdapter:  event.SourceAdapter,
		EventType:      event.EventType,
		Timestamp:      event.Timestamp,
		Actor: &Actor{
			ID:       event.SenderID,
			Nickname: event.ActorNickname,
			Role:     event.ActorRole,
		},
		Target: &Target{
			Type: adapterTargetType(event),
			ID:   adapterTargetID(event),
			Name: event.TargetName,
		},
		PayloadFields: event.PayloadFields,
		MessageID:     event.MessageID,
	}
	if event.PlainText != "" || len(event.Segments) > 0 {
		runtimeEvent.Message = &Message{
			PlainText: event.PlainText,
			Segments:  segmentsFromAdapter(event.Segments),
		}
	}
	return runtimeEvent
}

func segmentsFromAdapter(segments []MessageSegment) []MessageSegment {
	if len(segments) == 0 {
		return nil
	}
	projected := make([]MessageSegment, 0, len(segments))
	for _, seg := range segments {
		projected = append(projected, MessageSegment{
			Type: seg.Type,
			Data: seg.Data,
		})
	}
	return projected
}

// adapterTargetType prefers the explicit target an event names, falling back to
// the conversation it happened in. Only meta events carry an explicit target.
func adapterTargetType(event NormalizedEvent) string {
	if strings.TrimSpace(event.TargetType) != "" {
		return event.TargetType
	}
	return event.ConversationType
}

func adapterTargetID(event NormalizedEvent) string {
	if strings.TrimSpace(event.TargetID) != "" {
		return event.TargetID
	}
	return event.ConversationID
}
