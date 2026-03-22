package app

import (
	"container/list"
	"strings"
	"sync"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/outbound"
)

const defaultReplyTargetCacheSize = 10000

type replyTargetCache struct {
	mu    sync.Mutex
	limit int
	order *list.List
	items map[string]*list.Element
}

type replyTargetEntry struct {
	EventID string
	Target  outbound.ReplyTarget
}

func newReplyTargetCache(limit int) *replyTargetCache {
	if limit <= 0 {
		limit = defaultReplyTargetCacheSize
	}
	return &replyTargetCache{
		limit: limit,
		order: list.New(),
		items: make(map[string]*list.Element, limit),
	}
}

func (c *replyTargetCache) Record(event adapter.NormalizedEvent) {
	if c == nil {
		return
	}

	eventID := strings.TrimSpace(event.EventID)
	messageID := strings.TrimSpace(event.MessageID)
	targetType := strings.TrimSpace(event.ConversationType)
	targetID := strings.TrimSpace(event.ConversationID)
	if eventID == "" || messageID == "" || targetType == "" || targetID == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.items[eventID]; ok {
		existing.Value = replyTargetEntry{
			EventID: eventID,
			Target: outbound.ReplyTarget{
				MessageID:  messageID,
				TargetType: targetType,
				TargetID:   targetID,
			},
		}
		c.order.MoveToFront(existing)
		return
	}

	element := c.order.PushFront(replyTargetEntry{
		EventID: eventID,
		Target: outbound.ReplyTarget{
			MessageID:  messageID,
			TargetType: targetType,
			TargetID:   targetID,
		},
	})
	c.items[eventID] = element

	for c.order.Len() > c.limit {
		tail := c.order.Back()
		if tail == nil {
			return
		}
		entry, _ := tail.Value.(replyTargetEntry)
		delete(c.items, entry.EventID)
		c.order.Remove(tail)
	}
}

func (c *replyTargetCache) ResolveReplyTarget(eventID string) (outbound.ReplyTarget, bool) {
	if c == nil {
		return outbound.ReplyTarget{}, false
	}

	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return outbound.ReplyTarget{}, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.items[eventID]
	if !ok {
		return outbound.ReplyTarget{}, false
	}
	c.order.MoveToFront(element)
	entry, _ := element.Value.(replyTargetEntry)
	return entry.Target, true
}
