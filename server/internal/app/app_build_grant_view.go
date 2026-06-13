package app

func buildPluginGrantView(runtimeState *appRuntimeState, pluginStack appPlugins) *pluginGrantView {
	grantView := &pluginGrantView{
		state:           runtimeState,
		plugins:         pluginStack.Plugins,
		grantRepository: pluginStack.grantRepository,
	}
	pluginStack.Dispatcher.SetCapabilityChecker(grantView.capabilityGranted)
	return grantView
}
