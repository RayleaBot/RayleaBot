package catalog

import "github.com/RayleaBot/RayleaBot/server/internal/plugins"

func (c *Catalog) Subscribe(buffer int) (<-chan plugins.Snapshot, func()) {
	return c.hub.Subscribe(buffer)
}

func (c *Catalog) SubscriberCount() int {
	return c.hub.SubscriberCount()
}

func (c *Catalog) publish(snapshot plugins.Snapshot) {
	c.publishMany([]plugins.Snapshot{snapshot})
}

func (c *Catalog) publishMany(snapshots []plugins.Snapshot) {
	for _, snapshot := range snapshots {
		c.hub.PublishEach(func() plugins.Snapshot {
			return plugins.CloneSnapshot(snapshot)
		})
	}
}
