package cache

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

// Clear invalidates all cached entries.
func (c *IdentityCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.login = nil
	c.groups = make(map[string]*cachedGroupInfo)
	c.members = make(map[string]*cachedGroupMemberInfo)
	c.strangers = make(map[string]*cachedStrangerInfo)
}
