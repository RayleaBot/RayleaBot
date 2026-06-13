package adapter

import "time"

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
