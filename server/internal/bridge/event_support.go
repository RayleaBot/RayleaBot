package bridge

import (
	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
)

func isSupportedEvent(event adapterintake.NormalizedEvent) bool {
	if event.EventID == "" || event.SourceProtocol != "onebot11" || event.SourceAdapter != "adapter.onebot11" {
		return false
	}
	if event.Timestamp <= 0 || event.ConversationType == "" || event.ConversationID == "" || event.SenderID == "" {
		return false
	}
	if !isSupportedEventKind(event.Kind) {
		return false
	}
	if !isSupportedEventType(event) {
		return false
	}
	if isMessageEventKind(event.Kind) && event.PlainText == "" && len(event.Segments) == 0 {
		return false
	}
	return true
}

func isSupportedEventKind(kind string) bool {
	switch kind {
	case adapterintake.EventKindMessageText, adapterintake.EventKindMessage, adapterintake.EventKindMessageSent, adapterintake.EventKindNotice, adapterintake.EventKindRequest, adapterintake.EventKindMeta:
		return true
	default:
		return false
	}
}

func isMessageEventKind(kind string) bool {
	return kind == adapterintake.EventKindMessageText || kind == adapterintake.EventKindMessage || kind == adapterintake.EventKindMessageSent
}

func isSupportedEventType(event adapterintake.NormalizedEvent) bool {
	switch event.EventType {
	case "message.group":
		return event.ConversationType == "group"
	case "message.private":
		return event.ConversationType == "private"
	case "message_sent.group":
		return event.ConversationType == "group"
	case "message_sent.private":
		return event.ConversationType == "private"
	case "notice.member_increase",
		"notice.member_decrease",
		"notice.group_admin",
		"notice.group_ban",
		"notice.group_recall",
		"notice.group_upload",
		"notice.group_card",
		"notice.group_title",
		"notice.group_essence",
		"notice.group_message_emoji_like":
		return event.ConversationType == "group"
	case "notice.friend_add", "notice.friend_recall", "notice.profile_like", "notice.input_status":
		return event.ConversationType == "private"
	case "notice.poke", "notice.poke_recall", "notice.flash_file":
		return event.ConversationType == "group" || event.ConversationType == "private"
	case "request.friend":
		return event.ConversationType == "private"
	case "request.group":
		return event.ConversationType == "group"
	case "meta.heartbeat", "meta.lifecycle":
		return event.ConversationType == "system"
	default:
		return false
	}
}
