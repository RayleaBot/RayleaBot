package pluginwebhook

import (
	"sync"
	"time"
)

// replayCache is the in-memory LRU+TTL set the webhook service uses to
// detect duplicate (route, event_id) pairs within the replay tolerance
// window. Reads, writes, and eviction all live under a single mutex; the
// expected cardinality stays in the low thousands per route.
type replayCache struct {
	mu     sync.Mutex
	items  map[string]time.Time
}

func newReplayCache() *replayCache {
	return &replayCache{items: make(map[string]time.Time)}
}

// observe registers the given key as seen at observedAt. If the key has been
// seen and is still within its ttl it returns false (caller should reject
// the duplicate). Expired entries are dropped opportunistically so the map
// does not grow unbounded.
//
// observe is the legacy single-step API. Prefer peek + commit so dedup state
// is only mutated after authentication succeeds; otherwise a request that
// fails signature verification would still poison the dedup window for the
// genuine retry.
func (c *replayCache) observe(key string, observedAt time.Time, ttl time.Duration) bool {
	if c == nil {
		return true
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.purgeExpiredLocked(observedAt, ttl)

	if seenAt, ok := c.items[key]; ok {
		if observedAt.Sub(seenAt) <= ttl {
			return false
		}
	}
	c.items[key] = observedAt
	return true
}

// peek reports whether the given key would be treated as a duplicate at
// observedAt without mutating the cache. Use it for the read-only check
// before authentication.
func (c *replayCache) peek(key string, observedAt time.Time, ttl time.Duration) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.purgeExpiredLocked(observedAt, ttl)

	seenAt, ok := c.items[key]
	if !ok {
		return false
	}
	return observedAt.Sub(seenAt) <= ttl
}

// commit records the given key as seen at observedAt. Callers should invoke
// commit only after the request has passed all authentication checks.
func (c *replayCache) commit(key string, observedAt time.Time) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = observedAt
}

func (c *replayCache) purgeExpiredLocked(observedAt time.Time, ttl time.Duration) {
	for cached, seenAt := range c.items {
		if observedAt.Sub(seenAt) > ttl {
			delete(c.items, cached)
		}
	}
}

// Reset drops every cached entry. Intended for tests.
func (c *replayCache) Reset() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]time.Time)
}
