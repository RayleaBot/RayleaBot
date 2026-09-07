// Package chatevent holds the protocol-neutral shape an inbound chat event
// takes once an adapter has normalized it. Adapters own the wire formats; the
// event pipeline downstream of them speaks only these types.
package chatevent

import (
	"strconv"
	"strings"
)

// ScopedEventID makes an upstream event ID unique across adapter instances.
// The length prefix keeps arbitrary upstream IDs from colliding with the scope.
func ScopedEventID(adapterID, eventID string) string {
	adapterID = strings.TrimSpace(adapterID)
	if adapterID == "" || eventID == "" {
		return eventID
	}
	return strconv.Itoa(len(adapterID)) + ":" + adapterID + ":" + eventID
}

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

// Event families classify an event into the pipeline's coarse buckets,
// independent of which adapter produced it.
const (
	FamilyMessageText = "message_text"
	FamilyMessage     = "message"
	FamilyMessageSent = "message_sent"
	FamilyNotice      = "notice"
	FamilyRequest     = "request"
	FamilyMeta        = "meta"
)

// EventKind composes the kind an event reports, which is its source protocol
// and its family. The value is observable on the management WebSocket as
// last_supported_event_kind, so it names the protocol that actually produced
// the event rather than assuming one.
func EventKind(sourceProtocol, family string) string {
	return sourceProtocol + "." + family
}

// EventFamily recovers the family from a composed kind.
func EventFamily(kind string) string {
	if index := strings.LastIndex(kind, "."); index >= 0 {
		return kind[index+1:]
	}
	return kind
}

// OneBot11 kinds, spelled out because these exact strings are already
// published on the management surface.
const (
	EventKindMessageText = "onebot11." + FamilyMessageText
	EventKindMessage     = "onebot11." + FamilyMessage
	EventKindMessageSent = "onebot11." + FamilyMessageSent
	EventKindNotice      = "onebot11." + FamilyNotice
	EventKindRequest     = "onebot11." + FamilyRequest
	EventKindMeta        = "onebot11." + FamilyMeta
)

// Outbound message shapes. An adapter receives these and renders them onto its
// own wire format; nothing here names a protocol.
type OutboundMessageSend struct {
	// SourceAdapter names the adapter instance that must deliver this message,
	// and SourceProtocol the protocol it speaks. Both are empty when the caller
	// did not originate from a specific adapter, which resolves only while one
	// candidate is connected.
	SourceAdapter  string
	SourceProtocol string
	TargetType     string
	TargetID       string
	Segments       []MessageSegment
}

type OutboundMessageReply struct {
	SourceAdapter    string
	SourceProtocol   string
	TargetType       string
	TargetID         string
	ReplyToMessageID string
	Segments         []MessageSegment
}

// SendMessageResult reports the identifier the platform assigned to a message
// the adapter just sent.
type SendMessageResult struct {
	MessageID string
}
