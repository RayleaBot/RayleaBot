package intake

import (
	"fmt"
	"strings"
	"time"
)

func normalizeNoticeEvent(frame OneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	if frame.SelfID <= 0 {
		return NormalizedEvent{}, false
	}

	var eventType string
	conversationType := "group"
	conversationID := fmt.Sprintf("%d", frame.GroupID)
	senderID := fmt.Sprintf("%d", frame.UserID)
	switch frame.NoticeType {
	case "group_increase":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.member_increase"
	case "group_decrease":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.member_decrease"
	case "group_admin":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.group_admin"
	case "group_ban":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.group_ban"
	case "group_recall":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.group_recall"
	case "group_upload":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.group_upload"
	case "group_card":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.group_card"
	case "group_title":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.group_title"
	case "essence":
		if frame.UserID <= 0 || frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.group_essence"
	case "friend_add":
		if frame.UserID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.friend_add"
		conversationType = "private"
		conversationID = fmt.Sprintf("%d", frame.UserID)
	case "friend_recall":
		if frame.UserID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.friend_recall"
		conversationType = "private"
		conversationID = fmt.Sprintf("%d", frame.UserID)
	case "notify":
		return normalizeNotifyEvent(frame, observedAt)
	case "flash_file":
		if frame.UserID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "notice.flash_file"
		if frame.GroupID <= 0 {
			conversationType = "private"
			conversationID = fmt.Sprintf("%d", frame.UserID)
		}
	default:
		return NormalizedEvent{}, false
	}

	if conversationID == "0" || senderID == "0" {
		return NormalizedEvent{}, false
	}

	timestamp := frame.Time
	if timestamp <= 0 {
		timestamp = observedAt.Unix()
	}

	eventID := fmt.Sprintf("onebot11-notice-%s-%d-%d", strings.ReplaceAll(frame.NoticeType, "_", "-"), timestamp, frame.UserID)
	if frame.MessageID > 0 {
		eventID = fmt.Sprintf("onebot11-notice-%s-%d", strings.ReplaceAll(frame.NoticeType, "_", "-"), frame.MessageID)
	}

	payloadFields := buildCommonPayloadFields(frame)

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
		SenderID:         senderID,
		MessageID:        messageIDString(frame.MessageID),
		PayloadFields:    payloadFields,
	}, true
}
