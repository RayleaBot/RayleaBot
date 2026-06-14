package cache

import "time"

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
