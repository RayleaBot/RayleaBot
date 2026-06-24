package bridge

import (
	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	adaptersegments "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/segments"
	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

func bridgeEventLogAttrs(event adapterintake.NormalizedEvent) []any {
	attrs := []any{
		"direction", "inbound",
		"event_kind", event.Kind,
		"event_type", event.EventType,
		"event_timestamp", event.Timestamp,
		"conversation_type", event.ConversationType,
		"conversation_id", event.ConversationID,
		"sender_id", event.SenderID,
	}
	if event.BotID != "" {
		attrs = append(attrs, "self_id", event.BotID)
	}
	if event.TargetType != "" {
		attrs = append(attrs, "target_type", event.TargetType)
	}
	if event.TargetID != "" {
		attrs = append(attrs, "target_id", event.TargetID)
	}
	if event.TargetName != "" && event.ConversationType == "group" {
		attrs = append(attrs, "group_name", textsafe.SanitizeString(event.TargetName))
	}
	if event.MessageID != "" {
		attrs = append(attrs, "message_id", event.MessageID)
	}
	if event.PlainText != "" {
		attrs = append(attrs, "plain_text", event.PlainText)
	}
	if len(event.Segments) > 0 {
		attrs = append(attrs, "segments", bridgeSegmentsToAny(event.Segments))
	}
	if onebot := bridgeEventOneBotPayload(event); len(onebot) > 0 {
		if value, ok := onebot["post_type"]; ok {
			attrs = append(attrs, "post_type", value)
		}
		if value, ok := onebot["message_type"]; ok {
			attrs = append(attrs, "message_type", value)
		}
		if value, ok := onebot["time"]; ok {
			attrs = append(attrs, "time", value)
		}
		if value, ok := onebot["user_id"]; ok {
			attrs = append(attrs, "user_id", value)
		}
		if value, ok := onebot["group_id"]; ok {
			attrs = append(attrs, "group_id", value)
		}
		if value, ok := onebot["real_id"]; ok {
			attrs = append(attrs, "real_id", value)
		}
		if value, ok := onebot["message_seq"]; ok {
			attrs = append(attrs, "message_seq", value)
		}
		if value, ok := onebot["raw_message"]; ok {
			attrs = append(attrs, "raw_message", value)
		}
		if value, ok := onebot["message_format"]; ok {
			attrs = append(attrs, "message_format", value)
		}
		if value, ok := onebot["font"]; ok {
			attrs = append(attrs, "font", value)
		}
		if sender, ok := onebot["sender"].(map[string]any); ok && len(sender) > 0 {
			attrs = append(attrs, "sender", cloneBridgeData(sender))
			if value, ok := sender["nickname"]; ok {
				attrs = append(attrs, "sender_nickname", value)
			}
			if value, ok := sender["card"]; ok {
				attrs = append(attrs, "sender_card", value)
			}
			if value, ok := sender["role"]; ok {
				attrs = append(attrs, "sender_role", value)
			}
			if value, ok := sender["title"]; ok {
				attrs = append(attrs, "sender_title", value)
			}
		}
	}
	return attrs
}

func bridgeEventOneBotPayload(event adapterintake.NormalizedEvent) map[string]any {
	if event.PayloadFields == nil {
		return map[string]any{}
	}
	raw, ok := event.PayloadFields["onebot"].(map[string]any)
	if !ok || len(raw) == 0 {
		return map[string]any{}
	}
	return cloneBridgeData(raw)
}

func bridgeSegmentsToAny(segments []adaptersegments.MessageSegment) []any {
	items := make([]any, 0, len(segments))
	for _, segment := range segments {
		items = append(items, map[string]any{
			"type": segment.Type,
			"data": cloneBridgeData(segment.Data),
		})
	}
	return items
}

func cloneBridgeData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(data))
	for key, value := range data {
		cloned[key] = value
	}
	return cloned
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}
