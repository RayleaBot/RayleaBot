package lifecycle

import (
	"context"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func (c *Controller) RecoverFromDeadLetter(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	release, lockErr := c.acquireOperation(ctx, pluginID)
	if lockErr != nil {
		return plugins.Snapshot{}, lockErr
	}
	defer release()

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" {
		return plugins.Snapshot{}, plugins.ErrStateConflict
	}

	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager == nil {
		return plugins.Snapshot{}, plugins.ErrPluginNotInDeadLetter
	}
	if manager.Snapshot().State != pluginruntime.StateDeadLetter {
		return plugins.Snapshot{}, plugins.ErrPluginNotInDeadLetter
	}

	// Persist desired_state and update the catalog before mutating the
	// runtime manager. If persistence or catalog updates fail, the manager
	// must stay in dead_letter so a retry can pick the plugin up cleanly;
	// resetting the manager up front would leave the catalog reporting
	// dead_letter while the manager has already moved to stopped, which
	// would cause subsequent recovery attempts to fail with
	// plugin.not_in_dead_letter.
	updated := snapshot
	if snapshot.DesiredState != "enabled" {
		if err := persistPluginDesiredState(ctx, c.desiredStateRepo, pluginID, "enabled"); err != nil {
			return plugins.Snapshot{}, err
		}
		if reEnabled, setErr := c.plugins.SetDesiredState(pluginID, "enabled"); setErr == nil {
			updated = reEnabled
		} else {
			return updated, setErr
		}
	}

	manager.ResetCrashCount()
	manager.SetStopped()

	if startingSnapshot, runtimeErr := c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStarting)); runtimeErr == nil {
		updated = startingSnapshot
	} else {
		return updated, runtimeErr
	}

	if !c.launch(func() { c.startPluginAsync(updated.PluginID) }) {
		return updated, context.Canceled
	}
	c.reconcileRecoverySummaryBestEffort("plugin.dead_letter_recover")
	return updated, nil
}

func (c *Controller) handleCrash(pluginID string, crashCount int, _ string) {
	release, err := c.acquireOperation(c.lifecycleContext(), pluginID)
	if err != nil {
		return
	}
	defer release()
	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager == nil {
		return
	}
	current := manager.Snapshot()
	if current.State != pluginruntime.StateCrashed && current.State != pluginruntime.StateStopped {
		return
	}
	if current.CrashCount > 0 {
		crashCount = current.CrashCount
	}
	if c.dispatcher != nil {
		c.dispatcher.Deregister(pluginID)
	}
	c.clearBotIdentity(pluginID)

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		manager.SetStopped()
		c.publishRuntimeState(pluginID, string(pluginruntime.StateStopped))
		return
	}

	maxRetries := pluginruntime.DefaultMaxCrashRetries
	if crashCount >= maxRetries {
		c.enterDeadLetter(pluginID, snapshot, manager, crashCount, maxRetries)
		return
	}

	cfg := c.config().Runtime
	delay := pluginruntime.CrashBackoff(crashCount, cfg.CrashBackoffInitialSeconds, cfg.CrashBackoffMaxSeconds)
	nextRetry := time.Now().Add(delay)

	manager.SetBackoffState(nextRetry)
	c.publishRuntimeState(pluginID, string(pluginruntime.StateBackoff))

	if c.logger != nil {
		c.logger.Info(
			"插件异常退出，等待重启",
			"component", "app",
			"plugin_id", pluginID,
			"plugin_name", snapshot.Name,
			"crash_count", crashCount,
			"backoff_seconds", int(delay.Seconds()),
		)
	}

	c.launch(func() { c.backoffRestart(pluginID, delay, manager) })
}

func (c *Controller) enterDeadLetter(pluginID string, snapshot plugins.Snapshot, manager *pluginruntime.Manager, crashCount, maxRetries int) {
	manager.SetDeadLetterState()
	runtimeSnapshot := manager.Snapshot()
	c.publishRuntimeState(pluginID, string(pluginruntime.StateDeadLetter))
	if c.plugins != nil && runtimeSnapshot.EnteredDeadLetterAt != nil {
		if _, err := c.plugins.SetDeadLetterSnapshot(pluginID, plugins.DeadLetterSnapshot{
			EnteredAt:        *runtimeSnapshot.EnteredDeadLetterAt,
			CrashCount:       runtimeSnapshot.CrashCount,
			LastErrorCode:    runtimeSnapshot.LastErrorCode,
			LastErrorMessage: runtimeSnapshot.LastErrorMessage,
		}); err != nil {
			c.logLifecycleWarn("publish plugin dead letter state", pluginID, err)
		}
	}
	if c.logger != nil {
		c.logger.Warn(
			"插件连续异常退出，已停止自动重启，请检查后重新启用",
			"component", "app",
			"plugin_id", pluginID,
			"plugin_name", snapshot.Name,
			"crash_count", crashCount,
			"max_retries", maxRetries,
		)
	}
}

func (c *Controller) HandleCrash(pluginID string, crashCount int, reason string) {
	c.handleCrash(pluginID, crashCount, reason)
}

func (c *Controller) backoffRestart(pluginID string, delay time.Duration, expected *pluginruntime.Manager) {

	lifecycleCtx := c.lifecycleContext()
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-lifecycleCtx.Done():
		return
	case <-timer.C:
	}
	release, lockErr := c.acquireOperation(lifecycleCtx, pluginID)
	if lockErr != nil {
		return
	}
	defer release()
	manager, ok := c.runtimes.Get(pluginID)
	if !ok || manager != expected {
		return
	}

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		if manager, ok := c.runtimes.Get(pluginID); ok && manager != nil {
			manager.SetStopped()
		}
		c.publishRuntimeState(pluginID, string(pluginruntime.StateStopped))
		return
	}

	if manager.Snapshot().State != pluginruntime.StateBackoff {
		return
	}

	ctx, cancel := context.WithTimeout(lifecycleCtx, runtimeInitTimeout(c.config().Runtime))
	defer cancel()

	c.publishRuntimeState(pluginID, string(pluginruntime.StateStarting))
	if err := c.startRuntimeLocked(ctx, pluginID, manager); err != nil {
		c.logLifecycleWarn("restart plugin after crash backoff", pluginID, err)
		// startRuntime 可能在构建启动输入阶段失败（此时 Manager.Start 尚未执行），
		// manager 会停留在 backoff 状态，之后所有触发都视为等待重试而跳过启动；
		// 重置为 stopped，让下一次 scheduler 触发或管理操作能再次尝试。
		manager.SetStopped()
		c.publishRuntimeState(pluginID, string(pluginruntime.StateStopped))
	}
}
