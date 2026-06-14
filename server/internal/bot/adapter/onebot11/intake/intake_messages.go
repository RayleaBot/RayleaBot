package intake

import (
	"fmt"
	"strings"
	"time"

	adaptersegments "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/segments"
	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

func normalizeMessageEvent(frame OneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	return normalizeMessageLikeEvent(frame, observedAt, false)
}

func normalizeMessageSentEvent(frame OneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	return normalizeMessageLikeEvent(frame, observedAt, true)
}

func normalizeMessageLikeEvent(frame OneBotFrame, observedAt time.Time, sent bool) (NormalizedEvent, bool) {
	if frame.SelfID <= 0 || frame.UserID <= 0 {
		return NormalizedEvent{}, false
	}

	var eventType string
	var conversationType string
	var conversationID string
	switch frame.MessageType {
	case "private":
		if sent {
			eventType = "message_sent.private"
		} else {
			eventType = "message.private"
		}
		conversationType = "private"
		conversationID = fmt.Sprintf("%d", frame.UserID)
	case "group":
		if frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		if sent {
			eventType = "message_sent.group"
		} else {
			eventType = "message.group"
		}
		conversationType = "group"
		conversationID = fmt.Sprintf("%d", frame.GroupID)
	default:
		return NormalizedEvent{}, false
	}

	timestamp := frame.Time
	if timestamp <= 0 {
		timestamp = observedAt.Unix()
	}

	eventID := fmt.Sprintf("onebot11-message-%d-%d", timestamp, frame.UserID)
	if frame.MessageID > 0 {
		eventID = fmt.Sprintf("onebot11-message-%d", frame.MessageID)
	}
	if sent {
		eventID = fmt.Sprintf("onebot11-message-sent-%d-%d", timestamp, frame.UserID)
		if frame.MessageID > 0 {
			eventID = fmt.Sprintf("onebot11-message-sent-%d", frame.MessageID)
		}
	}

	segments := parseFrameMessage(frame)
	plainText := strings.TrimSpace(adaptersegments.ToPlainText(segments))
	if plainText == "" {
		plainText = strings.TrimSpace(textsafe.SanitizeString(frame.RawMessage))
	}
	if plainText == "" && len(segments) == 0 {
		return NormalizedEvent{}, false
	}

	var actorNickname, actorRole string
	if frame.Sender != nil {
		actorNickname = textsafe.SanitizeString(frame.Sender.Card)
		if actorNickname == "" {
			actorNickname = textsafe.SanitizeString(frame.Sender.Nickname)
		}
		actorRole = strings.TrimSpace(textsafe.SanitizeString(frame.Sender.Role))
	}

	var messageID string
	if frame.MessageID > 0 {
		messageID = fmt.Sprintf("%d", frame.MessageID)
	}

	payloadFields := buildCommonPayloadFields(frame)

	return NormalizedEvent{
		Kind: func() string {
			if sent {
				return EventKindMessageSent
			}
			return EventKindMessage
		}(),
		EventID:          eventID,
		BotID:            fmt.Sprintf("%d", frame.SelfID),
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        eventType,
		Timestamp:        timestamp,
		ConversationType: conversationType,
		ConversationID:   conversationID,
		SenderID:         fmt.Sprintf("%d", frame.UserID),
		PlainText:        plainText,
		Segments:         segments,
		MessageID:        messageID,
		ActorNickname:    actorNickname,
		ActorRole:        actorRole,
		PayloadFields:    payloadFields,
	}, true
}

// parseFrameMessage extracts segments from the OneBot frame Message field,
// falling back to CQ code parsing from RawMessage.
func parseFrameMessage(frame OneBotFrame) []adaptersegments.MessageSegment {
	if len(frame.Message) > 0 {
		trimmed := strings.TrimSpace(string(frame.Message))
		if len(trimmed) > 0 && trimmed[0] == '[' {
			if segments, err := adaptersegments.ParseMessageArray(frame.Message); err == nil && len(segments) > 0 {
				return segments
			}
		}
	}
	if frame.RawMessage != "" {
		return adaptersegments.ParseCQString(frame.RawMessage)
	}
	return nil
}
