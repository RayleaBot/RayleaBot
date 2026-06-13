package app

func (c *pluginLifecycleController) logLifecycleWarn(message, pluginID string, err error) {
	if c == nil || c.state == nil || c.state.Logger == nil || err == nil {
		return
	}

	c.state.Logger.Warn(
		message,
		"component", "app",
		"plugin_id", pluginID,
		"err", err.Error(),
	)
}
