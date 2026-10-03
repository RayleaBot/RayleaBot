package bridge

import (
	"log/slog"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/redact"
)

func bridgeEventLogAttrs(event chatevent.NormalizedEvent) []slog.Attr {
	attrs := make([]slog.Attr, 0, 30)
	attrs = append(attrs,
		slog.String("direction", "inbound"),
		slog.String("event_kind", event.Kind),
		slog.String("event_type", event.EventType),
		slog.Int64("event_timestamp", event.Timestamp),
		slog.String("conversation_type", event.ConversationType),
		slog.String("conversation_id", event.ConversationID),
		slog.String("sender_id", event.SenderID),
	)
	if event.BotID != "" {
		attrs = append(attrs, slog.String("self_id", event.BotID))
	}
	if nickname := strings.TrimSpace(redact.SanitizeString(event.BotNickname)); nickname != "" {
		attrs = append(attrs, slog.String("self_nickname", nickname))
	}
	if event.TargetType != "" {
		attrs = append(attrs, slog.String("target_type", event.TargetType))
	}
	if event.TargetID != "" {
		attrs = append(attrs, slog.String("target_id", event.TargetID))
	}
	if event.TargetName != "" && event.ConversationType == "group" {
		attrs = append(attrs, slog.String("group_name", redact.SanitizeString(event.TargetName)))
	}
	if event.MessageID != "" {
		attrs = append(attrs, slog.String("message_id", event.MessageID))
	}
	if event.PlainText != "" {
		attrs = append(attrs, slog.String("plain_text", event.PlainText))
	}
	if len(event.Segments) > 0 {
		attrs = append(attrs, slog.Any("segments", bridgeSegmentsToAny(event.Segments)))
	}
	if onebot := bridgeEventOneBotPayload(event); len(onebot) > 0 {
		if value, ok := onebot["post_type"]; ok {
			attrs = append(attrs, slog.Any("post_type", value))
		}
		if value, ok := onebot["message_type"]; ok {
			attrs = append(attrs, slog.Any("message_type", value))
		}
		if value, ok := onebot["time"]; ok {
			attrs = append(attrs, slog.Any("time", value))
		}
		if value, ok := onebot["user_id"]; ok {
			attrs = append(attrs, slog.Any("user_id", value))
		}
		if value, ok := onebot["group_id"]; ok {
			attrs = append(attrs, slog.Any("group_id", value))
		}
		if value, ok := onebot["real_id"]; ok {
			attrs = append(attrs, slog.Any("real_id", value))
		}
		if value, ok := onebot["message_seq"]; ok {
			attrs = append(attrs, slog.Any("message_seq", value))
		}
		if value, ok := onebot["raw_message"]; ok {
			attrs = append(attrs, slog.Any("raw_message", value))
		}
		if value, ok := onebot["message_format"]; ok {
			attrs = append(attrs, slog.Any("message_format", value))
		}
		if value, ok := onebot["font"]; ok {
			attrs = append(attrs, slog.Any("font", value))
		}
		if sender, ok := onebot["sender"].(map[string]any); ok && len(sender) > 0 {
			attrs = append(attrs, slog.Any("sender", cloneBridgeData(sender)))
			if value, ok := sender["nickname"]; ok {
				attrs = append(attrs, slog.Any("sender_nickname", value))
			}
			if value, ok := sender["card"]; ok {
				attrs = append(attrs, slog.Any("sender_card", value))
			}
			if value, ok := sender["role"]; ok {
				attrs = append(attrs, slog.Any("sender_role", value))
			}
			if value, ok := sender["title"]; ok {
				attrs = append(attrs, slog.Any("sender_title", value))
			}
		}
	}
	return attrs
}

func bridgeEventOneBotPayload(event chatevent.NormalizedEvent) map[string]any {
	if event.PayloadFields == nil {
		return map[string]any{}
	}
	raw, ok := event.PayloadFields["onebot"].(map[string]any)
	if !ok || len(raw) == 0 {
		return map[string]any{}
	}
	return cloneBridgeData(raw)
}

func bridgeSegmentsToAny(segments []chatevent.MessageSegment) []any {
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
