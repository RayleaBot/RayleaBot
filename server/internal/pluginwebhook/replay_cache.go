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
	mu    sync.Mutex
	items map[string]time.Time
}

func newReplayCache() *replayCache {
	return &replayCache{items: make(map[string]time.Time)}
}

// peek reports whether the given key would be treated as a duplicate at
// observedAt without mutating the cache. Use it for the read-only check
// before authentication; the authoritative duplicate decision is made
// later by commitIfAbsent under a single critical section.
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

// commitIfAbsent atomically checks for a live duplicate and, if none is
// found, records the key as seen at observedAt. It returns true when the
// caller is the unique winner for the (key, ttl) window and may proceed,
// and false when another concurrent request already won the slot.
//
// Callers must invoke commitIfAbsent only after authentication has
// succeeded. Splitting the duplicate check into peek + commit would leave
// a race where two authenticated callers both peek empty before either
// commits; commitIfAbsent collapses both steps under one lock so replay
// protection holds even under concurrent legitimate retries.
func (c *replayCache) commitIfAbsent(key string, observedAt time.Time, ttl time.Duration) bool {
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
