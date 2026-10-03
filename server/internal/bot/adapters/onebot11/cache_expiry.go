package onebot11

import (
	"container/heap"
	"time"
)

type cachedIdentity[T any] struct {
	value T
	identityExpiry
}

type identityExpiry struct {
	expiresAt time.Time
	key       string
	index     int
}

// A nil queue avoids heap maintenance until a cache first exceeds its limit.
// Once enabled, each live entry has exactly one indexed node, including after
// refreshes or invalidations. The owning IdentityCache holds mu for mutations.
type identityExpiryQueue []*identityExpiry

func (q identityExpiryQueue) Len() int { return len(q) }
func (q identityExpiryQueue) Less(i, j int) bool {
	return q[i].expiresAt.Before(q[j].expiresAt)
}
func (q identityExpiryQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index, q[j].index = i, j
}
func (q *identityExpiryQueue) Push(value any) {
	entry := value.(*identityExpiry)
	entry.index = len(*q)
	*q = append(*q, entry)
}
func (q *identityExpiryQueue) Pop() any {
	last := len(*q) - 1
	entry := (*q)[last]
	(*q)[last] = nil
	*q = (*q)[:last]
	entry.index = -1
	return entry
}

func setIdentityEntry[T any](entries map[string]*cachedIdentity[T], queue *identityExpiryQueue, key string, value T, now time.Time, ttl time.Duration) {
	entry, exists := entries[key]
	if !exists {
		entry = &cachedIdentity[T]{identityExpiry: identityExpiry{key: key}}
		entries[key] = entry
	}
	entry.value, entry.expiresAt = value, now.Add(ttl)
	if *queue != nil {
		if exists {
			heap.Fix(queue, entry.index)
		} else {
			heap.Push(queue, &entry.identityExpiry)
		}
	}
	if len(entries) <= identityCacheMaxEntries {
		return
	}
	if *queue == nil {
		*queue = make(identityExpiryQueue, 0, len(entries))
		for _, item := range entries {
			item.index = len(*queue)
			*queue = append(*queue, &item.identityExpiry)
		}
		heap.Init(queue)
	}
	// Overflow removes every expired entry before evicting live identities.
	for len(*queue) > 0 && now.After((*queue)[0].expiresAt) {
		delete(entries, heap.Pop(queue).(*identityExpiry).key)
	}
	for len(entries) > identityCacheMaxEntries {
		delete(entries, heap.Pop(queue).(*identityExpiry).key)
	}
}

func deleteIdentityEntry[T any](entries map[string]*cachedIdentity[T], queue *identityExpiryQueue, key string) {
	entry := entries[key]
	if entry == nil {
		return
	}
	if *queue != nil {
		heap.Remove(queue, entry.index)
	}
	delete(entries, key)
}
