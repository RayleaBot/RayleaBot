package qqofficial

import (
	"testing"
)

// Payload shapes below mirror live gateway captures. Identifiers, tokens and
// media URLs are fabricated: the real ones are per-user openids and signed
// media keys that must not land in a test snapshot.

const c2cTextDispatch = `{
  "author": {"bot": false, "id": "0000000000000000000000000000AAAA",
             "union_openid": "0000000000000000000000000000AAAA",
             "user_openid": "0000000000000000000000000000AAAA", "username": ""},
  "content": "hi",
  "id": "ROBOT1.0_fixture-c2c-message",
  "message_scene": {"ext": ["msg_idx=REFIDX_fixture"], "source": "default"},
  "message_type": 0,
  "timestamp": "2026-09-07T11:43:01+08:00"
}`

const groupAtDispatch = `{
  "author": {"bot": false, "id": "0000000000000000000000000000AAAA",
             "member_openid": "0000000000000000000000000000AAAA", "member_role": "owner",
             "union_openid": "0000000000000000000000000000AAAA", "username": "银蝶"},
  "content": " ",
  "group_id": "1111111111111111111111111111BBBB",
  "group_openid": "1111111111111111111111111111BBBB",
  "id": "ROBOT1.0_fixture-group-message",
  "message_scene": {"ext": ["msg_idx=REFIDX_fixture"], "source": "default"},
  "message_type": 0,
  "timestamp": "2026-09-07T11:43:45+08:00"
}`

const c2cImageDispatch = `{
  "attachments": [{"content_type": "image/jpeg", "filename": "photo.jpg",
                   "height": 1920, "size": 300218, "width": 1405,
                   "url": "https://multimedia.example.invalid/download?fileid=fixture"}],
  "author": {"bot": false, "id": "0000000000000000000000000000AAAA",
             "union_openid": "0000000000000000000000000000AAAA",
             "user_openid": "0000000000000000000000000000AAAA", "username": ""},
  "content": "<attachmentType=\"image/jpeg\",attachmentIndex=0,description=\"Zml4dHVyZQ==\">",
  "id": "ROBOT1.0_fixture-c2c-image",
  "timestamp": "2026-09-07T11:44:05+08:00"
}`

func TestNormalizeC2CMessageUsesThePeerAsTheConversation(t *testing.T) {
	t.Parallel()

	event, ok := NormalizeDispatch("C2C_MESSAGE_CREATE:fixture", dispatchC2CMessageCreate, []byte(c2cTextDispatch))
	if !ok {
		t.Fatal("C2C message was not normalized")
	}
	if event.EventType != "message.private" || event.ConversationType != "private" {
		t.Fatalf("event type/conversation = %q/%q, want message.private/private", event.EventType, event.ConversationType)
	}
	// A C2C dispatch carries no conversation identifier of its own.
	if event.ConversationID != "0000000000000000000000000000AAAA" || event.SenderID != event.ConversationID {
		t.Fatalf("conversation/sender = %q/%q, want both to be the peer openid", event.ConversationID, event.SenderID)
	}
	if event.SourceProtocol != SourceProtocol || event.SourceAdapter != SourceAdapter {
		t.Fatalf("source = %q/%q, want the qqofficial adapter", event.SourceProtocol, event.SourceAdapter)
	}
	if event.MessageID != "ROBOT1.0_fixture-c2c-message" {
		t.Fatalf("message id = %q, want the reply handle", event.MessageID)
	}
	if event.PlainText != "hi" {
		t.Fatalf("plain text = %q, want hi", event.PlainText)
	}
	if event.Timestamp != 1788752581 {
		t.Fatalf("timestamp = %d, want the RFC3339 value as unix seconds", event.Timestamp)
	}
	if len(event.Segments) != 1 || event.Segments[0].Type != "text" {
		t.Fatalf("segments = %+v, want a single text segment", event.Segments)
	}
}

func TestNormalizeGroupMentionDropsTheStrippedMention(t *testing.T) {
	t.Parallel()

	event, ok := NormalizeDispatch("GROUP_AT_MESSAGE_CREATE:fixture", dispatchGroupAtMessageCreate, []byte(groupAtDispatch))
	if !ok {
		t.Fatal("group mention was not normalized")
	}
	if event.EventType != "message.group" || event.ConversationType != "group" {
		t.Fatalf("event type/conversation = %q/%q, want message.group/group", event.EventType, event.ConversationType)
	}
	if event.ConversationID != "1111111111111111111111111111BBBB" {
		t.Fatalf("conversation = %q, want the group openid", event.ConversationID)
	}
	// The platform strips the mention before delivery, leaving whitespace that
	// must not be delivered as message text.
	if event.PlainText != "" {
		t.Fatalf("plain text = %q, want empty for a mention-only message", event.PlainText)
	}
	if len(event.Segments) != 0 {
		t.Fatalf("segments = %+v, want none for a mention-only message", event.Segments)
	}
	if event.ActorNickname != "银蝶" || event.ActorRole != "owner" {
		t.Fatalf("actor = %q/%q, want the group member identity", event.ActorNickname, event.ActorRole)
	}
}

func TestNormalizeAttachmentPromotesMediaOverThePlaceholder(t *testing.T) {
	t.Parallel()

	event, ok := NormalizeDispatch("", dispatchC2CMessageCreate, []byte(c2cImageDispatch))
	if !ok {
		t.Fatal("attachment message was not normalized")
	}
	// content is client-side markup describing the attachment, not text.
	if event.PlainText != "" || len(event.Segments) != 1 {
		t.Fatalf("segments = %+v (text %q), want only the image segment and no text", event.Segments, event.PlainText)
	}
	segment := event.Segments[0]
	if segment.Type != "image" {
		t.Fatalf("segment type = %q, want image", segment.Type)
	}
	if segment.Data["url"] != "https://multimedia.example.invalid/download?fileid=fixture" {
		t.Fatalf("segment url = %v, want the attachment url", segment.Data["url"])
	}
	if segment.Data["width"] != 1405 || segment.Data["height"] != 1920 {
		t.Fatalf("segment dimensions = %v x %v, want 1405 x 1920", segment.Data["width"], segment.Data["height"])
	}
	// An event id is always required downstream, so one is derived when the
	// gateway frame carries none.
	if event.EventID == "" {
		t.Fatal("event id was not derived from the message id")
	}
}

func TestNormalizeDispatchIgnoresUndeliveredKinds(t *testing.T) {
	t.Parallel()

	for _, dispatchType := range []string{"READY", "GROUP_ADD_ROBOT", "GROUP_DEL_ROBOT", "RESUMED"} {
		if _, ok := NormalizeDispatch("id", dispatchType, []byte(`{"group_openid":"g","timestamp":1788752606}`)); ok {
			t.Fatalf("%s was delivered, but it has no formal event type yet", dispatchType)
		}
	}
}

func TestParseDispatchTimestampAcceptsBothSpellings(t *testing.T) {
	t.Parallel()

	// Message dispatches use RFC3339; membership notices use a Unix integer.
	if got := parseDispatchTimestamp([]byte(`"2026-09-07T11:43:01+08:00"`)); got != 1788752581 {
		t.Fatalf("rfc3339 timestamp = %d, want 1788752581", got)
	}
	if got := parseDispatchTimestamp([]byte(`1788752606`)); got != 1788752606 {
		t.Fatalf("integer timestamp = %d, want 1788752606", got)
	}
	if got := parseDispatchTimestamp([]byte(`"not a time"`)); got != 0 {
		t.Fatalf("unparseable timestamp = %d, want 0", got)
	}
}
