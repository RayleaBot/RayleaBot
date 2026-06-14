package cache

import (
	"testing"
	"time"
)

func TestIdentityCacheTTLExpiry(t *testing.T) {
	t.Parallel()

	cache := NewIdentityCache(50 * time.Millisecond)

	cache.SetLogin(LoginInfo{ID: "1", Nickname: "Bot"})
	cache.SetGroupInfo("g1", GroupInfo{Name: "Group 1"})
	cache.SetGroupMemberInfo("g1", "u1", GroupMemberInfo{Role: "owner", Nickname: "User A", Card: "A"})
	cache.SetStrangerInfo("u2", StrangerInfo{Nickname: "User B"})

	if info, ok := cache.GetLogin(); !ok || info.ID != "1" {
		t.Fatalf("expected cached login, got ok=%v info=%+v", ok, info)
	}
	if info, ok := cache.GetGroupInfo("g1"); !ok || info.Name != "Group 1" {
		t.Fatalf("expected cached group info, got ok=%v info=%+v", ok, info)
	}
	if info, ok := cache.GetGroupMemberInfo("g1", "u1"); !ok || info.Role != "owner" {
		t.Fatalf("expected cached member info, got ok=%v info=%+v", ok, info)
	}
	if info, ok := cache.GetStrangerInfo("u2"); !ok || info.Nickname != "User B" {
		t.Fatalf("expected cached stranger info, got ok=%v info=%+v", ok, info)
	}

	time.Sleep(60 * time.Millisecond)

	if _, ok := cache.GetLogin(); ok {
		t.Fatal("expected login cache to be expired")
	}
	if _, ok := cache.GetGroupInfo("g1"); ok {
		t.Fatal("expected group info cache to be expired")
	}
	if _, ok := cache.GetGroupMemberInfo("g1", "u1"); ok {
		t.Fatal("expected member info cache to be expired")
	}
	if _, ok := cache.GetStrangerInfo("u2"); ok {
		t.Fatal("expected stranger info cache to be expired")
	}
}

func TestIdentityCacheClearInvalidatesAll(t *testing.T) {
	t.Parallel()

	cache := NewIdentityCache(10 * time.Minute)
	cache.SetLogin(LoginInfo{ID: "1", Nickname: "Bot"})
	cache.SetGroupInfo("g1", GroupInfo{Name: "Group"})
	cache.SetGroupMemberInfo("g1", "u1", GroupMemberInfo{Role: "member"})
	cache.SetStrangerInfo("u2", StrangerInfo{Nickname: "User B"})

	cache.Clear()

	if _, ok := cache.GetLogin(); ok {
		t.Fatal("expected login cache to be cleared")
	}
	if _, ok := cache.GetGroupInfo("g1"); ok {
		t.Fatal("expected group info cache to be cleared")
	}
	if _, ok := cache.GetGroupMemberInfo("g1", "u1"); ok {
		t.Fatal("expected member info cache to be cleared")
	}
	if _, ok := cache.GetStrangerInfo("u2"); ok {
		t.Fatal("expected stranger info cache to be cleared")
	}
}

func TestIdentityCacheInvalidatesSpecificGroupEntries(t *testing.T) {
	t.Parallel()

	cache := NewIdentityCache(10 * time.Minute)
	cache.SetGroupInfo("g1", GroupInfo{Name: "Group 1"})
	cache.SetGroupInfo("g2", GroupInfo{Name: "Group 2"})
	cache.SetGroupMemberInfo("g1", "u1", GroupMemberInfo{Role: "member"})
	cache.SetGroupMemberInfo("g1", "u2", GroupMemberInfo{Role: "admin"})
	cache.SetGroupMemberInfo("g2", "u1", GroupMemberInfo{Role: "owner"})

	cache.InvalidateGroupInfo("g1")
	if _, ok := cache.GetGroupInfo("g1"); ok {
		t.Fatal("expected g1 group info to be invalidated")
	}
	if info, ok := cache.GetGroupInfo("g2"); !ok || info.Name != "Group 2" {
		t.Fatalf("expected g2 group info to remain cached, got ok=%v info=%+v", ok, info)
	}

	cache.InvalidateGroupMemberInfo("g1", "u1")
	if _, ok := cache.GetGroupMemberInfo("g1", "u1"); ok {
		t.Fatal("expected g1/u1 member info to be invalidated")
	}
	if info, ok := cache.GetGroupMemberInfo("g1", "u2"); !ok || info.Role != "admin" {
		t.Fatalf("expected g1/u2 member info to remain cached, got ok=%v info=%+v", ok, info)
	}

	cache.InvalidateGroupMembers("g1")
	if _, ok := cache.GetGroupMemberInfo("g1", "u2"); ok {
		t.Fatal("expected remaining g1 member info to be invalidated")
	}
	if info, ok := cache.GetGroupMemberInfo("g2", "u1"); !ok || info.Role != "owner" {
		t.Fatalf("expected g2/u1 member info to remain cached, got ok=%v info=%+v", ok, info)
	}
}

func TestIdentityCacheInvalidatesFromEventFrameAndAPICall(t *testing.T) {
	t.Parallel()

	cache := NewIdentityCache(10 * time.Minute)
	cache.SetGroupInfo("100", GroupInfo{Name: "Group"})
	cache.SetGroupMemberInfo("100", "200", GroupMemberInfo{Role: "member"})
	cache.SetGroupMemberInfo("100", "201", GroupMemberInfo{Role: "admin"})

	cache.InvalidateForEvent(EventInvalidation{
		EventType:      "notice.group_card",
		ConversationID: "100",
		SenderID:       "200",
	})
	if _, ok := cache.GetGroupMemberInfo("100", "200"); ok {
		t.Fatal("expected event to invalidate one member")
	}
	if info, ok := cache.GetGroupMemberInfo("100", "201"); !ok || info.Role != "admin" {
		t.Fatalf("expected other member to remain cached, got ok=%v info=%+v", ok, info)
	}

	cache.InvalidateForFrame(FrameInvalidation{
		PostType:   "notice",
		NoticeType: "group_name",
		GroupID:    100,
	})
	if _, ok := cache.GetGroupInfo("100"); ok {
		t.Fatal("expected frame to invalidate group info")
	}

	cache.SetGroupMemberInfo("100", "201", GroupMemberInfo{Role: "admin"})
	cache.InvalidateForAPICall("set_group_admin", map[string]any{"group_id": "100"})
	if _, ok := cache.GetGroupMemberInfo("100", "201"); ok {
		t.Fatal("expected API call to invalidate group members")
	}
}
