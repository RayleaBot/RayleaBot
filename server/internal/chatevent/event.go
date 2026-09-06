// Package chatevent holds the protocol-neutral shape an inbound chat event
// takes once an adapter has normalized it. Adapters own the wire formats; the
// event pipeline downstream of them speaks only these types.
package chatevent

// NormalizedEvent is one inbound chat event after adapter normalization.
// SourceProtocol and SourceAdapter name the adapter that produced it, so
// consumers never infer origin from the other fields.
type NormalizedEvent struct {
	Kind             string
	EventID          string
	BotID            string
	SourceProtocol   string
	SourceAdapter    string
	EventType        string
	Timestamp        int64
	ConversationType string
	ConversationID   string
	SenderID         string
	TargetType       string
	TargetID         string
	PlainText        string
	Segments         []MessageSegment
	MessageID        string
	ActorNickname    string
	ActorRole        string
	TargetName       string
	PayloadFields    map[string]any
}

// MessageSegment is one structured piece of message content. The vocabulary of
// Type values is owned by the plugin protocol contract, not by any adapter.
type MessageSegment struct {
	Type string
	Data map[string]any
}

// Event kinds classify an event into the pipeline's coarse families. The
// values keep their historical onebot11 prefix because they are observable on
// the management WebSocket as last_supported_event_kind; renaming them is a
// contract change, not a refactor.
const (
	EventKindMessageText = "onebot11.message_text"
	EventKindMessage     = "onebot11.message"
	EventKindMessageSent = "onebot11.message_sent"
	EventKindNotice      = "onebot11.notice"
	EventKindRequest     = "onebot11.request"
	EventKindMeta        = "onebot11.meta"
)
