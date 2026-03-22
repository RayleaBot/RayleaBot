package adapter

import (
	"sync"
	"time"
)

// IdentityCache provides TTL-based caching for OneBot11 identity lookups
// (login info, group info, group member info, stranger info). Expired
// entries are detected on read; no background reaper is needed.
type IdentityCache struct {
	ttl       time.Duration
	mu        sync.RWMutex
	login     *cachedLogin
	groups    map[string]*cachedGroupInfo
	members   map[string]*cachedGroupMemberInfo
	strangers map[string]*cachedStrangerInfo
}

type cachedLogin struct {
	value     LoginInfo
	expiresAt time.Time
}

type cachedGroupInfo struct {
	value     GroupInfo
	expiresAt time.Time
}

type cachedGroupMemberInfo struct {
	value     GroupMemberInfo
	expiresAt time.Time
}

type cachedStrangerInfo struct {
	value     StrangerInfo
	expiresAt time.Time
}

// NewIdentityCache creates a new cache with the given entry TTL.
func NewIdentityCache(ttl time.Duration) *IdentityCache {
	return &IdentityCache{
		ttl:       ttl,
		groups:    make(map[string]*cachedGroupInfo),
		members:   make(map[string]*cachedGroupMemberInfo),
		strangers: make(map[string]*cachedStrangerInfo),
	}
}

// GetLogin returns the cached login info if present and not expired.
func (c *IdentityCache) GetLogin() (LoginInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.login == nil || time.Now().After(c.login.expiresAt) {
		return LoginInfo{}, false
	}
	return c.login.value, true
}

// SetLogin stores login info in the cache.
func (c *IdentityCache) SetLogin(info LoginInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.login = &cachedLogin{
		value:     info,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// GetGroupInfo returns the cached group info if present and not expired.
func (c *IdentityCache) GetGroupInfo(groupID string) (GroupInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry := c.groups[groupID]
	if entry == nil || time.Now().After(entry.expiresAt) {
		return GroupInfo{}, false
	}
	return entry.value, true
}

// SetGroupInfo stores group info in the cache.
func (c *IdentityCache) SetGroupInfo(groupID string, info GroupInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.groups[groupID] = &cachedGroupInfo{
		value:     info,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// GetGroupMemberInfo returns the cached group member info if present and
// not expired. The cache key combines groupID and userID.
func (c *IdentityCache) GetGroupMemberInfo(groupID, userID string) (GroupMemberInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := groupID + ":" + userID
	entry := c.members[key]
	if entry == nil || time.Now().After(entry.expiresAt) {
		return GroupMemberInfo{}, false
	}
	return entry.value, true
}

// SetGroupMemberInfo stores group member info in the cache.
func (c *IdentityCache) SetGroupMemberInfo(groupID, userID string, info GroupMemberInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := groupID + ":" + userID
	c.members[key] = &cachedGroupMemberInfo{
		value:     info,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// GetStrangerInfo returns the cached stranger info if present and not expired.
func (c *IdentityCache) GetStrangerInfo(userID string) (StrangerInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry := c.strangers[userID]
	if entry == nil || time.Now().After(entry.expiresAt) {
		return StrangerInfo{}, false
	}
	return entry.value, true
}

// SetStrangerInfo stores stranger info in the cache.
func (c *IdentityCache) SetStrangerInfo(userID string, info StrangerInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.strangers[userID] = &cachedStrangerInfo{
		value:     info,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Clear invalidates all cached entries.
func (c *IdentityCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.login = nil
	c.groups = make(map[string]*cachedGroupInfo)
	c.members = make(map[string]*cachedGroupMemberInfo)
	c.strangers = make(map[string]*cachedStrangerInfo)
}
