package plugins

// StopPluginFunc stops the runtime for the given plugin if it is running.
// It is injected by the app layer to avoid an import cycle with the runtime package.
type StopPluginFunc func(pluginID string)
