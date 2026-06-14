package lifecycle

func (c *Controller) logLifecycleWarn(message, pluginID string, err error) {
	if c == nil || c.logger == nil || err == nil {
		return
	}

	c.logger.Warn(
		message,
		"component", "app",
		"plugin_id", pluginID,
		"err", err.Error(),
	)
}
