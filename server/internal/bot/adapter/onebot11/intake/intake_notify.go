package intake

import (
	"fmt"
	"strings"
	"time"
)

func normalizeNotifyEvent(frame OneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	if frame.SelfID <= 0 || frame.UserID <= 0 {
		return NormalizedEvent{}, false
	}

	conversationType := "private"
	conversationID := fmt.Sprintf("%d", frame.UserID)
	if frame.GroupID > 0 {
		conversationType = "group"
		conversationID = fmt.Sprintf("%d", frame.GroupID)
	}

	var eventType string
	switch frame.SubType {
	case "poke":
		eventType = "notice.poke"
	case "poke_recall":
		eventType = "notice.poke_recall"
	case "profile_like":
		eventType = "notice.profile_like"
		conversationType = "private"
		conversationID = fmt.Sprintf("%d", frame.UserID)
	case "input_status":
		eventType = "notice.input_status"
	case "group_msg_emoji_like":
		eventType = "notice.group_message_emoji_like"
	default:
		return NormalizedEvent{}, false
	}

	timestamp := frame.Time
	if timestamp <= 0 {
		timestamp = observedAt.Unix()
	}

	payloadFields := buildCommonPayloadFields(frame)
	eventID := fmt.Sprintf("onebot11-notify-%s-%d-%d", strings.ReplaceAll(frame.SubType, "_", "-"), timestamp, frame.UserID)
	if frame.MessageID > 0 {
		eventID = fmt.Sprintf("onebot11-notify-%s-%d", strings.ReplaceAll(frame.SubType, "_", "-"), frame.MessageID)
	}

	return NormalizedEvent{
		Kind:             EventKindNotice,
		EventID:          eventID,
		BotID:            fmt.Sprintf("%d", frame.SelfID),
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        eventType,
		Timestamp:        timestamp,
		ConversationType: conversationType,
		ConversationID:   conversationID,
		SenderID:         fmt.Sprintf("%d", frame.UserID),
		MessageID:        messageIDString(frame.MessageID),
		PayloadFields:    payloadFields,
	}, true
}
