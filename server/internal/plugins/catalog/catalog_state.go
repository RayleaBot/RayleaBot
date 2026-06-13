package plugincatalog

import "github.com/RayleaBot/RayleaBot/server/internal/plugins"

func (c *Catalog) SetDesiredState(pluginID string, desired string) (plugins.Snapshot, error) {
	c.mu.Lock()

	entry, ok := c.items[pluginID]
	if !ok {
		c.mu.Unlock()
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}

	if entry.RegistrationState != "installed" {
		c.mu.Unlock()
		return plugins.Snapshot{}, plugins.ErrStateConflict
	}

	if entry.DesiredState == desired {
		c.mu.Unlock()
		return plugins.Snapshot{}, plugins.ErrStateConflict
	}

	entry.DesiredState = desired
	entry.DisplayState = plugins.DefaultDisplayState(entry)
	changed := pluginStateChanged(c.items[pluginID], entry)
	c.items[pluginID] = entry
	updated := plugins.CloneSnapshot(entry)
	c.mu.Unlock()

	if changed {
		c.publish(updated)
	}
	return updated, nil
}

func (c *Catalog) ApplyDesiredStates(states map[string]string) {
	if len(states) == 0 {
		return
	}

	c.mu.Lock()
	updated := make([]plugins.Snapshot, 0, len(states))

	for pluginID, desired := range states {
		entry, ok := c.items[pluginID]
		if !ok {
			continue
		}
		if entry.RegistrationState != "installed" {
			continue
		}
		if desired != "enabled" && desired != "disabled" {
			continue
		}

		current := entry
		entry.DesiredState = desired
		entry.DisplayState = plugins.DefaultDisplayState(entry)
		c.items[pluginID] = entry
		if pluginStateChanged(current, entry) {
			updated = append(updated, plugins.CloneSnapshot(entry))
		}
	}
	c.mu.Unlock()

	c.publishMany(updated)
}
