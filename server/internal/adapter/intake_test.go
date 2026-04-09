package adapter

import (
	"testing"
	"time"
)

func TestNormalizeSupportedEventGroupAdminNotice(t *testing.T) {
	t.Parallel()

	event, ok := normalizeSupportedEvent(oneBotFrame{
		PostType:   "notice",
		NoticeType: "group_admin",
		SubType:    "set",
		SelfID:     10001,
		UserID:     20001,
		GroupID:    30001,
		Time:       1710000001,
	}, time.Unix(1710000001, 0))
	if !ok {
		t.Fatal("expected group admin notice to normalize")
	}
	if event.Kind != EventKindNotice {
		t.Fatalf("unexpected kind: %q", event.Kind)
	}
	if event.EventType != "notice.group_admin" {
		t.Fatalf("unexpected event type: %q", event.EventType)
	}
	if event.ConversationType != "group" || event.ConversationID != "30001" {
		t.Fatalf("unexpected conversation: %#v", event)
	}
	if got := event.PayloadFields["sub_type"]; got != "set" {
		t.Fatalf("unexpected sub_type payload: %#v", got)
	}
}

func TestNormalizeSupportedEventFriendRequest(t *testing.T) {
	t.Parallel()

	event, ok := normalizeSupportedEvent(oneBotFrame{
		PostType:    "request",
		RequestType: "friend",
		SubType:     "add",
		SelfID:      10001,
		UserID:      20001,
		Time:        1710000002,
		Comment:     "请通过好友申请",
		Flag:        "friend-flag-1",
	}, time.Unix(1710000002, 0))
	if !ok {
		t.Fatal("expected friend request to normalize")
	}
	if event.Kind != EventKindRequest {
		t.Fatalf("unexpected kind: %q", event.Kind)
	}
	if event.EventType != "request.friend" {
		t.Fatalf("unexpected event type: %q", event.EventType)
	}
	if event.ConversationType != "private" || event.ConversationID != "20001" {
		t.Fatalf("unexpected conversation: %#v", event)
	}
	if got := event.PayloadFields["comment"]; got != "请通过好友申请" {
		t.Fatalf("unexpected comment payload: %#v", got)
	}
	if got := event.PayloadFields["flag"]; got != "friend-flag-1" {
		t.Fatalf("unexpected flag payload: %#v", got)
	}
}

func TestNormalizeSupportedEventMessageGroupCarriesOneBotFields(t *testing.T) {
	t.Parallel()

	event, ok := normalizeSupportedEvent(oneBotFrame{
		PostType:      "message",
		MessageType:   "group",
		SubType:       "normal",
		SelfID:        721011692,
		UserID:        721011692,
		GroupID:       860105388,
		Time:          1729679125,
		MessageID:     966671988,
		RealID:        966671988,
		MessageSeq:    966671988,
		RawMessage:    "您好",
		Font:          14,
		MessageFormat: "array",
		Message:       []byte(`[{"type":"text","data":{"text":"您好"}}]`),
		Sender: &senderObject{
			UserID:   721011692,
			Nickname: "--",
			Card:     "",
			Role:     "owner",
			Title:    "",
		},
	}, time.Unix(1729679125, 0))
	if !ok {
		t.Fatal("expected group message to normalize")
	}
	if event.EventType != "message.group" {
		t.Fatalf("unexpected event type: %q", event.EventType)
	}
	if event.MessageID != "966671988" {
		t.Fatalf("unexpected message id: %q", event.MessageID)
	}
	if event.ConversationID != "860105388" {
		t.Fatalf("unexpected conversation id: %q", event.ConversationID)
	}
	if event.Timestamp != 1729679125 {
		t.Fatalf("unexpected timestamp: %d", event.Timestamp)
	}
	onebot, ok := event.PayloadFields["onebot"].(map[string]any)
	if !ok {
		t.Fatalf("missing onebot payload: %#v", event.PayloadFields)
	}
	if got := onebot["group_id"]; got != "860105388" {
		t.Fatalf("unexpected group_id: %#v", got)
	}
	if got := onebot["message_id"]; got != "966671988" {
		t.Fatalf("unexpected message_id: %#v", got)
	}
	if got := onebot["real_id"]; got != "966671988" {
		t.Fatalf("unexpected real_id: %#v", got)
	}
	if got := onebot["message_seq"]; got != "966671988" {
		t.Fatalf("unexpected message_seq: %#v", got)
	}
	if got := onebot["time"]; got != int64(1729679125) {
		t.Fatalf("unexpected time: %#v", got)
	}
	if got := onebot["message_format"]; got != "array" {
		t.Fatalf("unexpected message_format: %#v", got)
	}
	if got := onebot["font"]; got != 14 {
		t.Fatalf("unexpected font: %#v", got)
	}
	sender, ok := onebot["sender"].(map[string]any)
	if !ok {
		t.Fatalf("missing sender payload: %#v", onebot["sender"])
	}
	if got := sender["nickname"]; got != "--" {
		t.Fatalf("unexpected sender nickname: %#v", got)
	}
	if got := sender["role"]; got != "owner" {
		t.Fatalf("unexpected sender role: %#v", got)
	}
}

func TestNormalizeSupportedEventMessageSentGroup(t *testing.T) {
	t.Parallel()

	event, ok := normalizeSupportedEvent(oneBotFrame{
		PostType:      "message_sent",
		MessageType:   "group",
		SubType:       "normal",
		SelfID:        721011692,
		UserID:        721011692,
		GroupID:       860105388,
		Time:          1729679125,
		MessageID:     966671988,
		RealID:        966671988,
		MessageSeq:    966671988,
		RawMessage:    "您好",
		Font:          14,
		MessageFormat: "array",
		Message:       []byte(`[{"type":"text","data":{"text":"您好"}}]`),
		Sender: &senderObject{
			UserID:   721011692,
			Nickname: "--",
			Role:     "owner",
		},
	}, time.Unix(1729679125, 0))
	if !ok {
		t.Fatal("expected message_sent group event to normalize")
	}
	if event.Kind != EventKindMessageSent {
		t.Fatalf("unexpected kind: %q", event.Kind)
	}
	if event.EventType != "message_sent.group" {
		t.Fatalf("unexpected event type: %q", event.EventType)
	}
	if event.ConversationType != "group" || event.ConversationID != "860105388" {
		t.Fatalf("unexpected conversation: %#v", event)
	}
	onebot, ok := event.PayloadFields["onebot"].(map[string]any)
	if !ok {
		t.Fatalf("missing onebot payload: %#v", event.PayloadFields)
	}
	if got := onebot["post_type"]; got != "message_sent" {
		t.Fatalf("unexpected post_type: %#v", got)
	}
}
