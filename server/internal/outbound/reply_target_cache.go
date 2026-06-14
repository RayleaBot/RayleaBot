package outbound

import (
	"container/list"
	"strings"
	"sync"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
)

const DefaultReplyTargetCacheSize = 10000

type ReplyTargetCache struct {
	mu    sync.Mutex
	limit int
	order *list.List
	items map[string]*list.Element
}

type replyTargetEntry struct {
	EventID string
	Target  ReplyTarget
}

func NewReplyTargetCache(limit int) *ReplyTargetCache {
	if limit <= 0 {
		limit = DefaultReplyTargetCacheSize
	}
	return &ReplyTargetCache{
		limit: limit,
		order: list.New(),
		items: make(map[string]*list.Element, limit),
	}
}

func (c *ReplyTargetCache) Record(event adapterintake.NormalizedEvent) {
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
			Target: ReplyTarget{
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
		Target: ReplyTarget{
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

func (c *ReplyTargetCache) ResolveReplyTarget(eventID string) (ReplyTarget, bool) {
	if c == nil {
		return ReplyTarget{}, false
	}

	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return ReplyTarget{}, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.items[eventID]
	if !ok {
		return ReplyTarget{}, false
	}
	c.order.MoveToFront(element)
	entry, _ := element.Value.(replyTargetEntry)
	return entry.Target, true
}
