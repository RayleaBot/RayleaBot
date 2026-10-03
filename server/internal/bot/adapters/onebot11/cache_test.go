package onebot11

import (
	"fmt"
	"strconv"
	"sync"
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

	expiredAt := time.Unix(0, 0)
	cache.mu.Lock()
	cache.login.expiresAt = expiredAt
	cache.groups["g1"].expiresAt = expiredAt
	cache.members["g1:u1"].expiresAt = expiredAt
	cache.strangers["u2"].expiresAt = expiredAt
	cache.mu.Unlock()

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

func TestIdentityCacheBoundsUnreadEntries(t *testing.T) {
	t.Parallel()

	cache := NewIdentityCache(time.Hour)
	for i := 0; i < identityCacheMaxEntries; i++ {
		cache.SetStrangerInfo(strconv.Itoa(i), StrangerInfo{Nickname: strconv.Itoa(i)})
	}
	cache.strangers["0"].expiresAt = time.Now().Add(-time.Minute)

	cache.SetStrangerInfo("newest", StrangerInfo{Nickname: "newest"})

	if got := len(cache.strangers); got != identityCacheMaxEntries {
		t.Fatalf("stranger entries = %d, want %d", got, identityCacheMaxEntries)
	}
	if _, ok := cache.strangers["0"]; ok {
		t.Fatal("the expired entry should have been evicted first")
	}
	if _, ok := cache.GetStrangerInfo("newest"); !ok {
		t.Fatal("the newly written entry must survive eviction")
	}

	for i := 0; i < 10; i++ {
		cache.SetStrangerInfo("extra-"+strconv.Itoa(i), StrangerInfo{})
	}
	if got := len(cache.strangers); got != identityCacheMaxEntries {
		t.Fatalf("stranger entries after overflow = %d, want %d", got, identityCacheMaxEntries)
	}
}

func TestIdentityCacheRefreshAndClearPreserveBoundedEntries(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"group", "member", "stranger"} {
		t.Run(kind, func(t *testing.T) {
			cache := NewIdentityCache(time.Hour)
			var set func(string, string)
			var get func(string) (string, bool)
			var invalidate func(string)
			var expiry func(string) *identityExpiry
			switch kind {
			case "group":
				set = func(key, name string) { cache.SetGroupInfo(key, GroupInfo{Name: name}) }
				get = func(key string) (string, bool) { info, ok := cache.GetGroupInfo(key); return info.Name, ok }
				invalidate = cache.InvalidateGroupInfo
				expiry = func(key string) *identityExpiry { return &cache.groups[key].identityExpiry }
			case "member":
				set = func(key, name string) { cache.SetGroupMemberInfo("g", key, GroupMemberInfo{Nickname: name}) }
				get = func(key string) (string, bool) {
					info, ok := cache.GetGroupMemberInfo("g", key)
					return info.Nickname, ok
				}
				invalidate = func(key string) { cache.InvalidateGroupMemberInfo("g", key) }
				expiry = func(key string) *identityExpiry { return &cache.members["g:"+key].identityExpiry }
			case "stranger":
				set = func(key, name string) { cache.SetStrangerInfo(key, StrangerInfo{Nickname: name}) }
				get = func(key string) (string, bool) { info, ok := cache.GetStrangerInfo(key); return info.Nickname, ok }
				expiry = func(key string) *identityExpiry { return &cache.strangers[key].identityExpiry }
			}
			for round := range 2 {
				for index := range identityCacheMaxEntries {
					set(strconv.Itoa(index), "original")
				}
				// Fixed deadlines distinguish the oldest entries even when the
				// platform clock returns the same instant for consecutive writes.
				cache.mu.Lock()
				oldest := time.Now().Add(30 * time.Minute)
				for index := range 4 {
					expiry(strconv.Itoa(index)).expiresAt = oldest.Add(time.Duration(index) * time.Minute)
				}
				cache.mu.Unlock()
				set("0", "refreshed")
				set("new", "new")
				if name, ok := get("0"); !ok || name != "refreshed" {
					t.Fatalf("round %d evicted refreshed entry: %q, %v", round, name, ok)
				}
				if _, ok := get("1"); ok {
					t.Fatal("oldest live entry survived overflow")
				}
				set("2", "refreshed again")
				set("extra", "extra")
				if name, ok := get("2"); !ok || name != "refreshed again" {
					t.Fatal("refresh after overflow lost its value or expiry")
				}
				if _, ok := get("3"); ok {
					t.Fatal("overflow evicted a newer entry")
				}
				if invalidate != nil {
					invalidate("0")
					if _, ok := get("0"); ok {
						t.Fatal("invalidated entry remained readable")
					}
					set("0", "replacement")
					if name, ok := get("0"); !ok || name != "replacement" {
						t.Fatal("invalidated entry could not be replaced")
					}
				}
				cache.Clear()
				if _, ok := get("new"); ok {
					t.Fatal("clear retained an entry after overflow")
				}
			}
		})
	}
}

func TestIdentityExpiryRemovesAllExpiredEntriesAfterRefresh(t *testing.T) {
	t.Parallel()
	entries := make(map[string]*cachedIdentity[GroupInfo])
	var queue identityExpiryQueue
	now := time.Now()
	for index := range identityCacheMaxEntries + 1 {
		setIdentityEntry(entries, &queue, strconv.Itoa(index), GroupInfo{}, now.Add(time.Duration(index)), time.Hour)
	}
	setIdentityEntry(entries, &queue, "1", GroupInfo{Name: "refreshed"}, now.Add(30*time.Minute), time.Hour)
	setIdentityEntry(entries, &queue, "new", GroupInfo{Name: "new"}, now.Add(time.Hour+time.Minute), time.Hour)
	if len(entries) != 2 || entries["1"] == nil || entries["new"] == nil {
		t.Fatalf("expired entries retained or refreshed identity lost: count=%d", len(entries))
	}
	// Expiration is strictly after the deadline, including when overflow triggers cleanup.
	boundaryEntries := make(map[string]*cachedIdentity[GroupInfo])
	var boundaryQueue identityExpiryQueue
	for index := range identityCacheMaxEntries + 1 {
		setIdentityEntry(boundaryEntries, &boundaryQueue, strconv.Itoa(index), GroupInfo{}, now, 0)
	}
	if len(boundaryEntries) != identityCacheMaxEntries {
		t.Fatalf("entries at their deadline were treated as expired: %d", len(boundaryEntries))
	}
	setIdentityEntry(boundaryEntries, &boundaryQueue, "expire-all", GroupInfo{}, now.Add(time.Second), -time.Second)
	if len(boundaryEntries) != 0 {
		t.Fatal("overflow retained expired identities")
	}
	setIdentityEntry(boundaryEntries, &boundaryQueue, "fresh", GroupInfo{Name: "fresh"}, now.Add(2*time.Second), time.Hour)
	if len(boundaryEntries) != 1 || boundaryEntries["fresh"] == nil {
		t.Fatal("fully expired cache could not accept another identity")
	}
}

func TestIdentityCacheBatchMemberInvalidationAfterOverflow(t *testing.T) {
	t.Parallel()
	cache := NewIdentityCache(time.Hour)
	for index := range identityCacheMaxEntries {
		group := "g"
		if index%2 != 0 {
			group = "g-other"
		}
		cache.SetGroupMemberInfo(group, strconv.Itoa(index), GroupMemberInfo{Nickname: "original"})
	}
	cache.mu.Lock()
	cache.members["g:0"].expiresAt = time.Now().Add(30 * time.Minute)
	cache.mu.Unlock()
	cache.SetGroupMemberInfo("g-other", "new", GroupMemberInfo{Nickname: "new"})
	cache.InvalidateGroupMemberInfo("g", "2")
	cache.InvalidateGroupMembers("g")
	for index := range identityCacheMaxEntries {
		group := "g"
		if index%2 != 0 {
			group = "g-other"
		}
		_, ok := cache.GetGroupMemberInfo(group, strconv.Itoa(index))
		if ok != (index%2 != 0) {
			t.Fatalf("group invalidation crossed boundaries: %s/%d, present=%v", group, index, ok)
		}
	}
	for index := range identityCacheMaxEntries {
		cache.SetGroupMemberInfo("g", strconv.Itoa(index), GroupMemberInfo{Nickname: "replacement"})
	}
	if info, ok := cache.GetGroupMemberInfo("g", strconv.Itoa(identityCacheMaxEntries-1)); !ok || info.Nickname != "replacement" {
		t.Fatal("batch invalidation prevented refilling the cache")
	}
}

func TestIdentityCacheConcurrentRefreshAndInvalidationAfterOverflow(t *testing.T) {
	t.Parallel()
	cache := NewIdentityCache(time.Hour)
	for index := range identityCacheMaxEntries + 1 {
		cache.SetGroupInfo(strconv.Itoa(index), GroupInfo{})
	}
	var workers sync.WaitGroup
	for worker := range 4 {
		workers.Go(func() {
			key := fmt.Sprintf("worker-%d", worker)
			for index := range 64 {
				cache.SetGroupInfo(key, GroupInfo{Name: strconv.Itoa(index)})
				cache.GetGroupInfo(key)
				cache.InvalidateGroupInfo(key)
			}
			cache.SetGroupInfo(key, GroupInfo{Name: "complete"})
		})
	}
	workers.Wait()
	for worker := range 4 {
		if info, ok := cache.GetGroupInfo(fmt.Sprintf("worker-%d", worker)); !ok || info.Name != "complete" {
			t.Fatal("concurrent invalidation lost another worker's final value")
		}
	}
}
