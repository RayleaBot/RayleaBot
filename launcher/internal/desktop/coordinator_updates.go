package desktop

import (
	"context"
	"errors"
	"os"
	"runtime"
)

func (c *Coordinator) CheckForUpdates() { go c.refreshRelease(true) }

func (c *Coordinator) refreshRelease(force bool) {
	if !c.releaseMu.TryLock() {
		return
	}
	defer c.releaseMu.Unlock()
	ctx, finish, allowed := c.updates.begin()
	if !allowed {
		return
	}
	defer finish()
	current := c.Snapshot().Launcher.ReleaseCheck
	current.Status, current.Summary = "checking", "正在检查更新。"
	current.CanCheck = false
	if current.CurrentVersion == "" {
		// The update check can wait on the network, while the installed version is already on disk.
		current.CurrentVersion = c.release.InstalledVersion()
	}
	c.publishRelease(current)
	result := c.release.getSnapshotContext(ctx, force, runtime.GOOS, runtime.GOARCH)
	if ctx.Err() == nil {
		c.publishRelease(result)
	}
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
	ctx, finish, allowed := c.updates.begin()
	if !allowed {
		return false
	}
	defer finish()
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
	cancelled := func() bool {
		release.Status, release.Summary, release.Detail = ReleaseCancelled, "更新已取消。", "可重新检查或安装更新。"
		release.ErrorCode, release.CanCheck = "launcher.update_cancelled", true
		c.publishRelease(release)
		return false
	}

	progress("正在下载更新。")
	prepared, err := c.release.downloadUpdate(ctx, runtime.GOOS, progress)
	if err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return cancelled()
		}
		return fail("launcher.update_download_failed", "下载更新失败。", "服务未受影响，请稍后重试或打开发布页手动下载。")
	}
	if prepared.Status == "up_to_date" {
		release.Status = ReleaseUpToDate
		release.UpdateAvailable = false
		release.CanCheck = true
		release.Summary = "当前已是所选范围内的最新版本。"
		c.publishRelease(release)
		return false
	}
	release.LatestVersion = prepared.Version

	progress("正在停止服务并安装更新。")
	unblockStartups := c.startups.block(false)
	defer unblockStartups()
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	if ctx.Err() != nil {
		return cancelled()
	}
	resumeService := c.process.IsRunning()
	if err := c.stopLocked(true, shutdownIntentUpdate); err != nil {
		if errors.Is(err, errStopCancelled) || ctx.Err() != nil {
			return cancelled()
		}
		return fail("launcher.update_apply_failed", "安装更新失败。", "服务未能停止，请手动停止服务后重试。")
	}
	if ctx.Err() != nil {
		return cancelled()
	}
	// Once replacement starts, let it complete even if exit is requested.
	// Shutdown still waits for the operation; interruption could leave partial files.
	if _, _, err := c.release.runServer(runtime.GOOS, updateApplyTimeout, "update", "apply", "--prepared", prepared.PreparedID); err != nil {
		return fail("launcher.update_apply_failed", "安装更新失败。", "请确认服务已停止后重试，或从发布页下载完整包解压覆盖安装目录。")
	}
	if ctx.Err() != nil {
		return cancelled()
	}
	if err := c.relaunch(c.release.basePath, runtime.GOOS, os.Getpid(), resumeService); err != nil {
		return fail("launcher.update_relaunch_failed", "更新已安装，但启动器未能自动重启。", "请手动重新打开启动器。")
	}
	return true
}
