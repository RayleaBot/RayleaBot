package intake

import (
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

func normalizeRequestEvent(frame OneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	if frame.SelfID <= 0 || frame.UserID <= 0 {
		return NormalizedEvent{}, false
	}

	var (
		eventType        string
		conversationType string
		conversationID   string
	)
	switch frame.RequestType {
	case "friend":
		eventType = "request.friend"
		conversationType = "private"
		conversationID = fmt.Sprintf("%d", frame.UserID)
	case "group":
		if frame.GroupID <= 0 {
			return NormalizedEvent{}, false
		}
		eventType = "request.group"
		conversationType = "group"
		conversationID = fmt.Sprintf("%d", frame.GroupID)
	default:
		return NormalizedEvent{}, false
	}

	timestamp := frame.Time
	if timestamp <= 0 {
		timestamp = observedAt.Unix()
	}

	eventID := fmt.Sprintf("onebot11-request-%s-%d-%d", strings.ReplaceAll(frame.RequestType, "_", "-"), timestamp, frame.UserID)
	payloadFields := buildCommonPayloadFields(frame)
	if comment := strings.TrimSpace(textsafe.SanitizeString(frame.Comment)); comment != "" {
		payloadFields["comment"] = comment
	}
	if flag := strings.TrimSpace(textsafe.SanitizeString(frame.Flag)); flag != "" {
		payloadFields["flag"] = flag
	}

	return NormalizedEvent{
		Kind:             EventKindRequest,
		EventID:          eventID,
		BotID:            fmt.Sprintf("%d", frame.SelfID),
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        eventType,
		Timestamp:        timestamp,
		ConversationType: conversationType,
		ConversationID:   conversationID,
		SenderID:         fmt.Sprintf("%d", frame.UserID),
		PayloadFields:    payloadFields,
	}, true
}

func normalizeMetaEvent(frame OneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	if frame.SelfID <= 0 {
		return NormalizedEvent{}, false
	}

	var eventType string
	switch frame.MetaEventType {
	case "heartbeat":
		eventType = "meta.heartbeat"
	case "lifecycle":
		eventType = "meta.lifecycle"
	default:
		return NormalizedEvent{}, false
	}

	timestamp := frame.Time
	if timestamp <= 0 {
		timestamp = observedAt.Unix()
	}

	eventID := fmt.Sprintf("onebot11-meta-%s-%d", strings.ReplaceAll(frame.MetaEventType, "_", "-"), timestamp)
	if subType := strings.TrimSpace(frame.SubType); subType != "" {
		eventID = fmt.Sprintf("onebot11-meta-%s-%s-%d", strings.ReplaceAll(frame.MetaEventType, "_", "-"), strings.ReplaceAll(subType, "_", "-"), timestamp)
	}

	botID := fmt.Sprintf("%d", frame.SelfID)
	return NormalizedEvent{
		Kind:             EventKindMeta,
		EventID:          eventID,
		BotID:            botID,
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        eventType,
		Timestamp:        timestamp,
		ConversationType: "system",
		ConversationID:   "bot:" + botID,
		SenderID:         botID,
		TargetType:       "bot",
		TargetID:         botID,
		PayloadFields:    buildCommonPayloadFields(frame),
	}, true
}
