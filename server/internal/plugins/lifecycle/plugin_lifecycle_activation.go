package lifecycle

import "github.com/RayleaBot/RayleaBot/server/internal/plugins"

func (c *Controller) declaredCapabilities(snapshot plugins.Snapshot) []string {
	return plugins.DedupeCapabilities(snapshot.DeclaredCapabilities)
}
