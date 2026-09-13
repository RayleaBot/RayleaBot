package desktop

func (c *Coordinator) CheckForUpdates() { go c.refreshRelease(true) }

func (c *Coordinator) refreshRelease(force bool) {
	if !c.releaseMu.TryLock() {
		return
	}
	defer c.releaseMu.Unlock()
	current := c.Snapshot().Launcher.ReleaseCheck
	current.Status, current.Summary = "checking", "正在检查更新。"
	current.CanCheck = false
	c.publishRelease(current)
	c.publishRelease(c.release.GetSnapshot(force))
}
