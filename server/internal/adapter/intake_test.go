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
