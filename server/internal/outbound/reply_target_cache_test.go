package outbound

import (
	"testing"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
)

func TestReplyTargetCacheStoresRecentEventTargets(t *testing.T) {
	t.Parallel()

	cache := NewReplyTargetCache(2)
	cache.Record(adapterintake.NormalizedEvent{
		EventID:          "evt-1",
		MessageID:        "msg-1",
		ConversationType: "group",
		ConversationID:   "2001",
	})
	cache.Record(adapterintake.NormalizedEvent{
		EventID:          "evt-2",
		MessageID:        "msg-2",
		ConversationType: "private",
		ConversationID:   "3001",
	})
	cache.Record(adapterintake.NormalizedEvent{
		EventID:          "evt-3",
		MessageID:        "msg-3",
		ConversationType: "group",
		ConversationID:   "2002",
	})

	if _, ok := cache.ResolveReplyTarget("evt-1"); ok {
		t.Fatal("expected oldest entry to be evicted")
	}

	target, ok := cache.ResolveReplyTarget("evt-3")
	if !ok {
		t.Fatal("expected latest entry to be present")
	}
	if target.MessageID != "msg-3" || target.TargetType != "group" || target.TargetID != "2002" {
		t.Fatalf("unexpected cached target: %#v", target)
	}
}
