package shell

import (
	adaptercache "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/cache"
	"testing"
	"time"
)

func TestIdentityCacheTTLExpiry(t *testing.T) {

	t.Parallel()

	cache := adaptercache.NewIdentityCache(50 * time.Millisecond)

	cache.SetLogin(adaptercache.LoginInfo{ID: "1", Nickname: "Bot"})
	if info, ok := cache.GetLogin(); !ok || info.ID != "1" {
		t.Fatalf("expected cached login, got ok=%v info=%+v", ok, info)
	}

	cache.SetGroupInfo("g1", adaptercache.GroupInfo{Name: "Group 1"})
	if info, ok := cache.GetGroupInfo("g1"); !ok || info.Name != "Group 1" {
		t.Fatalf("expected cached group info, got ok=%v info=%+v", ok, info)
	}

	cache.SetGroupMemberInfo("g1", "u1", adaptercache.GroupMemberInfo{Role: "owner", Nickname: "测试用户A", Card: "A"})
	if info, ok := cache.GetGroupMemberInfo("g1", "u1"); !ok || info.Role != "owner" {
		t.Fatalf("expected cached member info, got ok=%v info=%+v", ok, info)
	}

	cache.SetStrangerInfo("u2", adaptercache.StrangerInfo{Nickname: "测试用户B"})
	if info, ok := cache.GetStrangerInfo("u2"); !ok || info.Nickname != "测试用户B" {
		t.Fatalf("expected cached stranger info, got ok=%v info=%+v", ok, info)
	}

	// Wait for TTL expiry.
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

	cache := adaptercache.NewIdentityCache(10 * time.Minute)

	cache.SetLogin(adaptercache.LoginInfo{ID: "1", Nickname: "Bot"})
	cache.SetGroupInfo("g1", adaptercache.GroupInfo{Name: "Group"})
	cache.SetGroupMemberInfo("g1", "u1", adaptercache.GroupMemberInfo{Role: "member"})
	cache.SetStrangerInfo("u2", adaptercache.StrangerInfo{Nickname: "测试用户B"})

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

	cache := adaptercache.NewIdentityCache(10 * time.Minute)
	cache.SetGroupInfo("g1", adaptercache.GroupInfo{Name: "Group 1"})
	cache.SetGroupInfo("g2", adaptercache.GroupInfo{Name: "Group 2"})
	cache.SetGroupMemberInfo("g1", "u1", adaptercache.GroupMemberInfo{Role: "member"})
	cache.SetGroupMemberInfo("g1", "u2", adaptercache.GroupMemberInfo{Role: "admin"})
	cache.SetGroupMemberInfo("g2", "u1", adaptercache.GroupMemberInfo{Role: "owner"})

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
