package desktop

import (
	"errors"
	"fmt"
	"os"
)

func (c *Coordinator) CheckForUpdates() { go c.refreshRelease(true) }

func (c *Coordinator) DownloadUpdate() { go c.downloadRelease() }

func (c *Coordinator) InstallDownloadedUpdate() error {
	// Acquire the release lane before the operation lane so a queued install
	// cannot block stop or shutdown while a check or download is still active.
	c.releaseMu.Lock()
	defer c.releaseMu.Unlock()
	c.operationMu.Lock()
	defer c.operationMu.Unlock()
	launcherSnapshot := c.Snapshot().Launcher
	releaseCheck := launcherSnapshot.ReleaseCheck
	if releaseCheck.Status == "installing" {
		return nil
	}
	if !releaseReadyForInstall(releaseCheck) {
		return errors.New("没有已验证且可安装的更新")
	}
	if launcherSnapshot.ProcessOwnership == "external" {
		return errors.New("检测到外部服务正在运行，请先停止服务后再安装更新")
	}
	serviceWasRunning := c.process.IsRunning()
	if serviceWasRunning {
		if err := c.stopLocked(true); err != nil {
			return err
		}
	}
	operation, err := c.operationContext()
	if err != nil {
		return err
	}
	result, err := c.release.Install(os.Getpid(), serviceWasRunning, operation.resolvedSettings)
	c.publishRelease(result)
	if err != nil && serviceWasRunning {
		return c.recoverServiceAfterUpdateFailure(err)
	}
	return err
}

func releaseReadyForInstall(snapshot ReleaseCheckSnapshot) bool {
	return snapshot.CanInstall && (snapshot.Status == "ready_to_install" || snapshot.Status == "failed")
}

func (c *Coordinator) recoverServiceAfterUpdateFailure(installErr error) error {
	startupContext, finishStartup, allowed := c.startups.begin()
	if !allowed {
		return fmt.Errorf("更新助手启动失败，且原服务恢复失败：恢复启动被阻止；更新错误：%w", installErr)
	}
	defer finishStartup()

	recoveryErr := c.startLocked(startupContext)
	if recoveryErr == nil {
		if !serviceAvailable(c.Snapshot()) {
			recoveryErr = errors.New("服务未恢复到可用状态")
		}
	}
	if recoveryErr != nil {
		return fmt.Errorf("更新助手启动失败，且原服务恢复失败：%v；更新错误：%w", recoveryErr, installErr)
	}
	return installErr
}

func (c *Coordinator) refreshRelease(force bool) {
	if !c.releaseMu.TryLock() {
		return
	}
	defer c.releaseMu.Unlock()
	current := c.Snapshot().Launcher.ReleaseCheck
	if current.Status == "downloading" || current.Status == "installing" {
		return
	}
	if !force && current.Status == "ready_to_install" {
		return
	}
	current.Status, current.Summary = "checking", "正在检查更新。"
	current.CanCheck, current.CanDownload, current.CanInstall = false, false, false
	c.publishRelease(current)
	c.publishRelease(c.release.GetSnapshot(force))
}

func (c *Coordinator) downloadRelease() {
	if !c.releaseMu.TryLock() {
		return
	}
	defer c.releaseMu.Unlock()
	current := c.Snapshot().Launcher.ReleaseCheck
	if current.Status == "checking" || current.Status == "downloading" || current.Status == "installing" {
		return
	}
	current.Status, current.Summary = "downloading", "正在下载更新。"
	current.CanCheck, current.CanDownload, current.CanInstall = false, false, false
	c.publishRelease(current)
	c.publishRelease(c.release.Download())
}
