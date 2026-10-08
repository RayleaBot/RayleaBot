package qqofficial

import (
	"container/list"
	"sync"
	"time"
)

const dispatchHistoryLimit = 4096
const dispatchHistoryTTL = 10 * time.Minute

// dispatchHistory deduplicates the two group dispatch names across reconnects.
type dispatchHistory struct {
	mu    sync.Mutex
	items map[string]*list.Element
	order list.List
}

type dispatchHistoryEntry struct {
	id      string
	expires time.Time
}

func (h *dispatchHistory) duplicate(id string, now time.Time) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.items == nil {
		h.items = make(map[string]*list.Element)
	}
	for head := h.order.Front(); head != nil; head = h.order.Front() {
		entry := head.Value.(dispatchHistoryEntry)
		if now.Before(entry.expires) {
			break
		}
		delete(h.items, entry.id)
		h.order.Remove(head)
	}
	if _, exists := h.items[id]; exists {
		return true
	}
	if h.order.Len() == dispatchHistoryLimit {
		head := h.order.Front()
		delete(h.items, head.Value.(dispatchHistoryEntry).id)
		h.order.Remove(head)
	}
	h.items[id] = h.order.PushBack(dispatchHistoryEntry{id: id, expires: now.Add(dispatchHistoryTTL)})
	return false
}
