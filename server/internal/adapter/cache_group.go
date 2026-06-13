package adapter

import "time"

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

func (c *IdentityCache) InvalidateGroupInfo(groupID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.groups, groupID)
}
