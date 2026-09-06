package runtime

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
)

// EventFromAdapter projects a normalized adapter event onto the plugin runtime
// event shape. It is the single conversion used by every delivery path, so the
// bridge and the builtin menu cannot drift apart on target resolution.
func EventFromAdapter(event chatevent.NormalizedEvent) Event {
	runtimeEvent := Event{
		EventID:        event.EventID,
		SourceProtocol: event.SourceProtocol,
		SourceAdapter:  event.SourceAdapter,
		EventType:      event.EventType,
		Timestamp:      event.Timestamp,
		Actor: &EventActor{
			ID:       event.SenderID,
			Nickname: event.ActorNickname,
			Role:     event.ActorRole,
		},
		Target: &EventTarget{
			Type: adapterTargetType(event),
			ID:   adapterTargetID(event),
			Name: event.TargetName,
		},
		PayloadFields: event.PayloadFields,
		MessageID:     event.MessageID,
	}
	if event.PlainText != "" || len(event.Segments) > 0 {
		runtimeEvent.Message = &EventMessage{
			PlainText: event.PlainText,
			Segments:  segmentsFromAdapter(event.Segments),
		}
	}
	return runtimeEvent
}

func segmentsFromAdapter(segments []chatevent.MessageSegment) []EventSegment {
	if len(segments) == 0 {
		return nil
	}
	projected := make([]EventSegment, 0, len(segments))
	for _, seg := range segments {
		projected = append(projected, EventSegment{
			Type: seg.Type,
			Data: seg.Data,
		})
	}
	return projected
}

// adapterTargetType prefers the explicit target an event names, falling back to
// the conversation it happened in. Only meta events carry an explicit target.
func adapterTargetType(event chatevent.NormalizedEvent) string {
	if strings.TrimSpace(event.TargetType) != "" {
		return event.TargetType
	}
	return event.ConversationType
}

func adapterTargetID(event chatevent.NormalizedEvent) string {
	if strings.TrimSpace(event.TargetID) != "" {
		return event.TargetID
	}
	return event.ConversationID
}
