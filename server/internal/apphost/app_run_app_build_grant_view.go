package apphost

import pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"

func buildPluginGrantView(runtimeState *appRuntimeState, pluginStack appPlugins) *pluginservice.GrantView {
	grantView := pluginservice.NewGrantView(pluginservice.GrantViewDeps{
		Plugins:               pluginStack.Plugins,
		GrantRepository:       pluginStack.grantRepository,
		AutoGrantCapabilities: currentPluginAutoGrantCapabilities(runtimeState),
	})
	pluginStack.Dispatcher.SetCapabilityChecker(grantView.CapabilityGranted)
	return grantView
}
