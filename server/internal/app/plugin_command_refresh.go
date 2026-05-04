package app

import (
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func applicationRefreshPluginCommands(catalog *plugins.Catalog, dispatcher *dispatch.Dispatcher, pluginID string, settings map[string]any) {
	if catalog == nil {
		return
	}

	snapshot, ok := catalog.RefreshCommands(pluginID, settings)
	if !ok || dispatcher == nil {
		return
	}
	dispatcher.UpdateCommands(pluginID, dispatchCommands(snapshot.Commands))
}
