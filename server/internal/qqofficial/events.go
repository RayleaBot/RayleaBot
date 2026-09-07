package qqofficial

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
)

// Gateway dispatch names this adapter understands.
const (
	dispatchC2CMessageCreate     = "C2C_MESSAGE_CREATE"
	dispatchGroupAtMessageCreate = "GROUP_AT_MESSAGE_CREATE"
)

const (
	SourceProtocol = "qqofficial"
	SourceAdapter  = "adapter.qqofficial"
)

type dispatchAuthor struct {
	ID           string `json:"id"`
	UserOpenID   string `json:"user_openid"`
	MemberOpenID string `json:"member_openid"`
	UnionOpenID  string `json:"union_openid"`
	MemberRole   string `json:"member_role"`
	Username     string `json:"username"`
	Bot          bool   `json:"bot"`
}

type dispatchAttachment struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Filename    string `json:"filename"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Size        int    `json:"size"`
}

type dispatchMessage struct {
	ID          string               `json:"id"`
	Content     string               `json:"content"`
	GroupOpenID string               `json:"group_openid"`
	Author      dispatchAuthor       `json:"author"`
	Attachments []dispatchAttachment `json:"attachments"`
	Timestamp   json.RawMessage      `json:"timestamp"`
}

// NormalizeDispatch converts one gateway dispatch frame into the neutral event
// shape. It reports false for dispatches this adapter does not deliver, which
// includes lifecycle frames such as READY and the group membership notices that
// have no formal event type yet.
func NormalizeDispatch(eventID, dispatchType string, data []byte) (chatevent.NormalizedEvent, bool) {
	switch dispatchType {
	case dispatchC2CMessageCreate, dispatchGroupAtMessageCreate:
	default:
		return chatevent.NormalizedEvent{}, false
	}

	var message dispatchMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return chatevent.NormalizedEvent{}, false
	}

	senderID := messageSenderID(message.Author)
	if senderID == "" || strings.TrimSpace(message.ID) == "" {
		return chatevent.NormalizedEvent{}, false
	}

	event := chatevent.NormalizedEvent{
		Kind:           chatevent.EventKindMessage,
		EventID:        strings.TrimSpace(eventID),
		SourceProtocol: SourceProtocol,
		SourceAdapter:  SourceAdapter,
		Timestamp:      parseDispatchTimestamp(message.Timestamp),
		SenderID:       senderID,
		ActorNickname:  strings.TrimSpace(message.Author.Username),
		ActorRole:      strings.TrimSpace(message.Author.MemberRole),
		MessageID:      message.ID,
		// The at-mention is already stripped by the platform, so a mention-only
		// message arrives as whitespace and must not read as text.
		PlainText: messagePlainText(message.Content),
	}
	if event.EventID == "" {
		event.EventID = dispatchType + ":" + message.ID
	}

	switch dispatchType {
	case dispatchGroupAtMessageCreate:
		group := strings.TrimSpace(message.GroupOpenID)
		if group == "" {
			return chatevent.NormalizedEvent{}, false
		}
		event.EventType = "message.group"
		event.ConversationType = "group"
		event.ConversationID = group
	default:
		// A C2C conversation has no identifier of its own: the peer is the
		// conversation.
		event.EventType = "message.private"
		event.ConversationType = "private"
		event.ConversationID = senderID
	}

	event.Segments = messageSegments(message)
	event.PayloadFields = map[string]any{
		"qq_official": qqOfficialPayload(dispatchType, message),
	}
	return event, true
}

// messageSenderID prefers the scoped openid for the surface the message came
// from, falling back to the generic author id.
// messagePlainText drops content that is not text a person sent: whitespace
// left by a stripped mention, and the attachment placeholder markup.
func messagePlainText(content string) string {
	if isAttachmentPlaceholder(content) {
		return ""
	}
	return strings.TrimSpace(content)
}

func messageSenderID(author dispatchAuthor) string {
	for _, candidate := range []string{author.MemberOpenID, author.UserOpenID, author.ID, author.UnionOpenID} {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// messageSegments projects text and attachments onto the frozen segment
// vocabulary. The platform sends no segment array: content is plain text, and
// rich media rides in a parallel attachments list.
func messageSegments(message dispatchMessage) []chatevent.MessageSegment {
	segments := make([]chatevent.MessageSegment, 0, 1+len(message.Attachments))
	if text := strings.TrimSpace(message.Content); text != "" && !isAttachmentPlaceholder(message.Content) {
		segments = append(segments, chatevent.MessageSegment{
			Type: "text",
			Data: map[string]any{"text": text},
		})
	}
	for _, attachment := range message.Attachments {
		url := strings.TrimSpace(attachment.URL)
		if url == "" {
			continue
		}
		data := map[string]any{"url": url}
		if attachment.Filename != "" {
			data["file"] = attachment.Filename
		}
		if attachment.ContentType != "" {
			data["content_type"] = attachment.ContentType
		}
		if attachment.Width > 0 {
			data["width"] = attachment.Width
		}
		if attachment.Height > 0 {
			data["height"] = attachment.Height
		}
		if attachment.Size > 0 {
			data["size"] = attachment.Size
		}
		segments = append(segments, chatevent.MessageSegment{Type: attachmentSegmentType(attachment), Data: data})
	}
	if len(segments) == 0 {
		return nil
	}
	return segments
}

// isAttachmentPlaceholder reports the pseudo-tag the platform puts in content
// when the message body is an attachment. It is markup for the official client,
// not text anyone sent.
func isAttachmentPlaceholder(content string) bool {
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, `<attachmentType=`) && strings.HasSuffix(trimmed, ">")
}

func attachmentSegmentType(attachment dispatchAttachment) string {
	switch {
	case strings.HasPrefix(attachment.ContentType, "image/"):
		return "image"
	case strings.HasPrefix(attachment.ContentType, "video/"):
		return "video"
	case strings.HasPrefix(attachment.ContentType, "audio/"), strings.HasPrefix(attachment.ContentType, "voice/"):
		return "record"
	default:
		return "file"
	}
}

func qqOfficialPayload(dispatchType string, message dispatchMessage) map[string]any {
	payload := map[string]any{
		"dispatch_type": dispatchType,
		"message_id":    message.ID,
	}
	if group := strings.TrimSpace(message.GroupOpenID); group != "" {
		payload["group_openid"] = group
	}
	if id := strings.TrimSpace(message.Author.UserOpenID); id != "" {
		payload["user_openid"] = id
	}
	if id := strings.TrimSpace(message.Author.MemberOpenID); id != "" {
		payload["member_openid"] = id
	}
	if role := strings.TrimSpace(message.Author.MemberRole); role != "" {
		payload["member_role"] = role
	}
	return payload
}

// parseDispatchTimestamp accepts both spellings the gateway uses: message
// dispatches carry an RFC3339 string, membership notices a Unix integer.
func parseDispatchTimestamp(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var seconds int64
	if err := json.Unmarshal(raw, &seconds); err == nil {
		return seconds
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0
	}
	if parsed, err := time.Parse(time.RFC3339, text); err == nil {
		return parsed.Unix()
	}
	if parsed, err := strconv.ParseInt(text, 10, 64); err == nil {
		return parsed
	}
	return 0
}
