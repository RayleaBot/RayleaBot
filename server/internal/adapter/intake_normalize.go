package adapter

import "time"

func normalizeSupportedEvent(frame oneBotFrame, observedAt time.Time) (NormalizedEvent, bool) {
	switch frame.PostType {
	case "message":
		return normalizeMessageEvent(frame, observedAt)
	case "message_sent":
		return normalizeMessageSentEvent(frame, observedAt)
	case "notice":
		return normalizeNoticeEvent(frame, observedAt)
	case "request":
		return normalizeRequestEvent(frame, observedAt)
	case "meta_event":
		return normalizeMetaEvent(frame, observedAt)
	default:
		return NormalizedEvent{}, false
	}
}
