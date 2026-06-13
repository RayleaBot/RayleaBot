package plugincatalog

import "github.com/RayleaBot/RayleaBot/server/internal/plugins"

func (c *Catalog) Subscribe(buffer int) (<-chan plugins.Snapshot, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan plugins.Snapshot, buffer)
	c.mu.Lock()
	id := c.nextSubID
	c.nextSubID++
	c.subscribers[id] = ch
	c.mu.Unlock()

	return ch, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		subscriber, ok := c.subscribers[id]
		if !ok {
			return
		}
		delete(c.subscribers, id)
		close(subscriber)
	}
}

func (c *Catalog) SubscriberCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.subscribers)
}

func (c *Catalog) publish(snapshot plugins.Snapshot) {
	c.publishMany([]plugins.Snapshot{snapshot})
}

func (c *Catalog) publishMany(snapshots []plugins.Snapshot) {
	if len(snapshots) == 0 {
		return
	}

	c.mu.RLock()
	subscribers := make([]chan plugins.Snapshot, 0, len(c.subscribers))
	for _, subscriber := range c.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	c.mu.RUnlock()

	for _, snapshot := range snapshots {
		for _, subscriber := range subscribers {
			select {
			case subscriber <- plugins.CloneSnapshot(snapshot):
			default:
			}
		}
	}
}
