package desktop

import (
	"os"
	"runtime"
)

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

// ApplyUpdate downloads and installs the available release through the server
// CLI, then starts the updated Launcher. It returns true when this process
// should exit so the new Launcher can take the single-instance lock. Failures
// are not rolled back; the snapshot offers a retry and the release page.
func (c *Coordinator) ApplyUpdate() bool {
	if !c.releaseMu.TryLock() {
		return false
	}
	defer c.releaseMu.Unlock()
	release := c.Snapshot().Launcher.ReleaseCheck
	if !release.UpdateAvailable {
		return false
	}
	progress := func(summary string) {
		release.Status, release.Summary, release.Detail, release.ErrorCode, release.CanCheck = ReleaseUpdating, summary, "", "", false
		c.publishRelease(release)
	}
	fail := func(code, summary, detail string) bool {
		release.Status, release.Summary, release.Detail, release.ErrorCode, release.CanCheck = ReleaseFailed, summary, detail, code, true
		c.publishRelease(release)
		return false
	}

	progress("正在下载更新。")
	if _, _, err := c.release.runServer(runtime.GOOS, updateDownloadTimeout, "update", "download"); err != nil {
		return fail("launcher.update_download_failed", "下载更新失败。", "服务未受影响，请稍后重试或打开发布页手动下载。")
	}

	progress("正在停止服务并安装更新。")
	unblockStartups := c.startups.block(false)
	defer unblockStartups()
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	if err := c.stopLocked(true); err != nil {
		return fail("launcher.update_apply_failed", "安装更新失败。", "服务未能停止，请手动停止服务后重试。")
	}
	if _, _, err := c.release.runServer(runtime.GOOS, updateApplyTimeout, "update", "apply"); err != nil {
		return fail("launcher.update_apply_failed", "安装更新失败。", "请确认服务已停止后重试，或从发布页下载完整包解压覆盖安装目录。")
	}
	if err := startDetachedLauncher(c.release.basePath, runtime.GOOS, os.Getpid()); err != nil {
		return fail("launcher.update_relaunch_failed", "更新已安装。", "请手动重新打开启动器。")
	}
	return true
}
