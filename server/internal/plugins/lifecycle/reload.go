package lifecycle

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/settings"
)

func (c *Controller) Reload(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	release, lockErr := c.acquireOperation(ctx, pluginID)
	if lockErr != nil {
		return plugins.Snapshot{}, lockErr
	}
	defer release()

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	if snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" {
		return plugins.Snapshot{}, plugins.ErrStateConflict
	}

	if c.refreshManifest != nil {
		refreshed, err := c.refreshManifest(ctx, pluginID)
		if err != nil {
			return plugins.Snapshot{}, err
		}
		snapshot = refreshed
		if snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" {
			return plugins.Snapshot{}, plugins.ErrStateConflict
		}
		if c.webhooks != nil {
			c.webhooks.SyncSnapshots(c.plugins.List())
		}
	}

	if c.syncRenderTemplates != nil {
		if err := c.syncRenderTemplates(ctx); err != nil {
			return plugins.Snapshot{}, err
		}
	}

	updated, err := c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStarting))
	if err != nil {
		return plugins.Snapshot{}, err
	}

	taskID := c.createReloadTask(pluginID, snapshot)
	if !c.launch(func() { c.reloadPluginAsync(pluginID, taskID) }) {
		c.failReloadTaskForError(taskID, pluginID, context.Canceled, "插件重载已取消")
		return updated, context.Canceled
	}
	c.reconcileRecoverySummaryBestEffort("plugin.reload")
	return updated, nil
}

func (c *Controller) reloadPluginAsync(pluginID, taskID string) {
	c.startReloadTask(taskID)

	ctx, cancel := c.lifecycleTimeoutContext(runtimeInitTimeout(c.config().Runtime))
	defer cancel()
	release, lockErr := c.acquireOperation(ctx, pluginID)
	if lockErr != nil {
		c.failReloadTaskForError(taskID, pluginID, lockErr, "插件重载已取消")
		return
	}
	defer release()

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		c.publishRuntimeState(pluginID, string(pluginruntime.StateStopped))
		c.failReloadTask(taskID, pluginID, errorcodes.PlatformInvalidRequest, "插件当前不可重载")
		return
	}
	current, ok := c.runtimes.Get(pluginID)
	if !ok || current == nil {
		c.updateReloadTask(taskID, 30, "启动插件运行时")
		manager := c.runtimes.GetOrCreate(pluginID)
		if err := c.startRuntimeLocked(ctx, pluginID, manager); err != nil {
			c.logLifecycleWarn("start plugin runtime during reload", pluginID, err)
			c.publishRuntimeFailure(pluginID, err)
			c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
			return
		}
		c.finishReloadTask(taskID, pluginID)
		return
	}

	switch current.Snapshot().State {
	case pluginruntime.StateStopped:
		c.updateReloadTask(taskID, 30, "启动插件运行时")
		if err := c.startRuntimeLocked(ctx, pluginID, current); err != nil {
			c.logLifecycleWarn("start stopped plugin runtime during reload", pluginID, err)
			c.publishRuntimeFailure(pluginID, err)
			c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
			return
		}
		c.finishReloadTask(taskID, pluginID)
		return
	case pluginruntime.StateBackoff, pluginruntime.StateCrashed, pluginruntime.StateDeadLetter:
		current.ResetCrashCount()
		current.SetStopped()
		c.updateReloadTask(taskID, 30, "重置插件运行时")
		if err := c.startRuntimeLocked(ctx, pluginID, current); err != nil {
			c.logLifecycleWarn("restart plugin runtime during reload", pluginID, err)
			c.publishRuntimeFailure(pluginID, err)
			c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
			return
		}
		c.finishReloadTask(taskID, pluginID)
		return
	case pluginruntime.StateStarting, pluginruntime.StateStopping:
		c.failReloadTask(taskID, pluginID, errorcodes.PlatformInvalidRequest, "插件运行时正在切换状态")
		return
	}

	c.updateReloadTask(taskID, 30, "构建插件运行时")
	spec, payload, err := c.buildStartInputs(ctx, pluginID)
	if err != nil {
		c.logLifecycleWarn("build runtime spec for plugin reload", pluginID, err)
		c.publishRuntimeState(pluginID, string(pluginruntime.StateRunning))
		c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
		return
	}

	newManager := c.runtimes.NewDetached()
	c.updateReloadTask(taskID, 60, "重载插件运行时")
	if err := newManager.Start(ctx, spec, payload); err != nil {
		c.runtimes.ReleaseRetired(newManager)
		c.logLifecycleWarn("reload plugin runtime", pluginID, err)
		c.publishRuntimeState(pluginID, string(pluginruntime.StateRunning))
		c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
		return
	}

	var retired *dispatch.Drain
	activated := false
	activationErr := c.settings.Activate(ctx, pluginID, payload.Config, func() error {
		latest, _ := c.plugins.Get(pluginID)
		var swapErr error
		retired, swapErr = c.dispatcher.SwapPlugin(pluginID, newManager, spec.Events, latest.Commands, spec.EffectiveConcurrency)
		if swapErr != nil {
			return swapErr
		}
		c.runtimes.Replace(pluginID, newManager)
		activated = true
		return nil
	})
	if !activated {
		stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), max(spec.ShutdownGrace, time.Second))
		activationErr = errors.Join(activationErr, newManager.Stop(stopCtx))
		cancel()
		c.runtimes.ReleaseRetired(newManager)
		c.failReloadTaskForError(taskID, pluginID, activationErr, "插件重载失败")
		return
	}
	newManager.ResetCrashCount()
	c.publishRuntimeState(pluginID, string(pluginruntime.StateRunning))
	c.clearBotIdentity(pluginID)
	c.afterRuntimeRegistered(ctx, pluginID, payload.Bots)
	if retired != nil {
		drainCtx, cancelDrain := context.WithTimeout(c.lifecycleContext(), spec.ShutdownGrace)
		if err := retired.Wait(drainCtx); err != nil {
			activationErr = errors.Join(activationErr, err)
			c.logLifecycleWarn("old plugin delivery drain canceled", pluginID, err)
		}
		cancelDrain()
	}
	// Use a fresh budget: an expired drain must not prevent process cleanup.
	stopCtx, cancelStop := context.WithTimeout(context.WithoutCancel(c.lifecycleContext()), max(spec.ShutdownGrace, time.Second))
	if err := current.Stop(stopCtx); err != nil {
		activationErr = errors.Join(activationErr, err)
		c.logLifecycleWarn("stop old plugin runtime after reload", pluginID, err)
	}
	cancelStop()
	c.runtimes.ReleaseRetired(current)
	if activationErr != nil {
		c.failReloadTaskForError(taskID, pluginID, activationErr, "插件切换已完成，后续处理失败")
		return
	}
	c.finishReloadTask(taskID, pluginID)
}

func (c *Controller) failReloadTaskForError(taskID string, pluginID string, err error, fallbackMessage string) {
	var applyErr *settings.ApplyError
	if errors.As(err, &applyErr) {
		definition, _ := errorcodes.Lookup(errorcodes.PluginSettingsApplyFailed)
		c.failReloadTask(taskID, pluginID, errorcodes.PluginSettingsApplyFailed, definition.Message, applyErr.Details())
		return
	}
	code := errorcodes.PluginInternalError
	message := fallbackMessage

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		code = errorcodes.PlatformTaskTimeout
		message = "插件重载超时"
	}

	var runtimeErr *plugins.Error
	if errors.As(err, &runtimeErr) {
		if strings.TrimSpace(runtimeErr.Code) != "" {
			code = runtimeErr.Code
		}
		if strings.TrimSpace(runtimeErr.Message) != "" {
			message = runtimeErr.Message
		}
	} else if strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}

	c.failReloadTask(taskID, pluginID, code, message)
}
