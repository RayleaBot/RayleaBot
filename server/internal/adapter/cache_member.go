package adapter

import "time"

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

func (c *IdentityCache) InvalidateGroupMemberInfo(groupID, userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.members, groupID+":"+userID)
}

func (c *IdentityCache) InvalidateGroupMembers(groupID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := groupID + ":"
	for key := range c.members {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.members, key)
		}
	}
}
