package lifecycle

import (
	"context"
	"errors"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func (c *Controller) Reload(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
	if c == nil || c.plugins == nil {
		return plugins.Snapshot{}, errors.New("plugin lifecycle controller is not available")
	}

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
	}

	if c.syncRenderTemplates != nil {
		if err := c.syncRenderTemplates(ctx); err != nil {
			return plugins.Snapshot{}, err
		}
	}

	updated, err := c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStarting))
	if err != nil {
		updated = snapshot
	}

	taskID := c.createReloadTask(pluginID, snapshot)
	go c.reloadPluginAsync(pluginID, c.currentBotID(), taskID)
	c.reconcileRecoverySummaryBestEffort("plugin.reload")
	return updated, nil
}

func (c *Controller) reloadPluginAsync(pluginID, botID string, taskID string) {
	if c == nil {
		return
	}
	botID = strings.TrimSpace(botID)
	c.startReloadTask(taskID)

	ctx, cancel := c.lifecycleTimeoutContext(runtimeInitTimeout(c.config().Runtime))
	defer cancel()

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStopped))
		c.failReloadTask(taskID, pluginID, "platform.invalid_request", "插件当前不可重载")
		return
	}
	current, ok := c.runtimes.Get(pluginID)
	if !ok || current == nil {
		c.updateReloadTask(taskID, 30, "启动插件运行时")
		manager := c.runtimes.GetOrCreate(pluginID)
		if err := c.startRuntime(ctx, pluginID, botID, manager); err != nil {
			c.logLifecycleWarn("start plugin runtime during reload", pluginID, err)
			_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStopped))
			c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
			return
		}
		c.finishReloadTask(taskID, pluginID)
		return
	}

	switch current.Snapshot().State {
	case pluginruntime.StateStopped:
		c.updateReloadTask(taskID, 30, "启动插件运行时")
		if err := c.startRuntime(ctx, pluginID, botID, current); err != nil {
			c.logLifecycleWarn("start stopped plugin runtime during reload", pluginID, err)
			_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStopped))
			c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
			return
		}
		c.finishReloadTask(taskID, pluginID)
		return
	case pluginruntime.StateBackoff, pluginruntime.StateCrashed, pluginruntime.StateDeadLetter:
		current.ResetCrashCount()
		current.SetStopped()
		c.updateReloadTask(taskID, 30, "重置插件运行时")
		if err := c.startRuntime(ctx, pluginID, botID, current); err != nil {
			c.logLifecycleWarn("restart plugin runtime during reload", pluginID, err)
			_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStopped))
			c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
			return
		}
		c.finishReloadTask(taskID, pluginID)
		return
	case pluginruntime.StateStarting, pluginruntime.StateStopping:
		c.failReloadTask(taskID, pluginID, "platform.invalid_request", "插件运行时正在切换状态")
		return
	}

	c.updateReloadTask(taskID, 30, "构建插件运行时")
	spec, payload, err := c.buildStartInputs(ctx, pluginID, botID)
	if err != nil {
		c.logLifecycleWarn("build runtime spec for plugin reload", pluginID, err)
		_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateStopped))
		c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
		return
	}

	newManager := c.runtimes.NewDetached()
	c.updateReloadTask(taskID, 60, "重载插件运行时")
	if err := c.dispatcher.ReloadPlugin(ctx, pluginID, current, newManager, spec, payload, dispatchCommands(snapshot.Commands)); err != nil {
		c.logLifecycleWarn("reload plugin runtime", pluginID, err)
		_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateRunning))
		c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
		return
	}

	c.runtimes.Replace(pluginID, newManager)
	newManager.ResetCrashCount()
	_, _ = c.plugins.SetRuntimeState(pluginID, string(pluginruntime.StateRunning))
	c.clearBotIdentity(pluginID)
	c.afterRuntimeRegistered(ctx, pluginID, botID)
	c.finishReloadTask(taskID, pluginID)
}

func (c *Controller) failReloadTaskForError(taskID string, pluginID string, err error, fallbackMessage string) {
	code := "plugin.internal_error"
	message := fallbackMessage

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		code = "platform.task_timeout"
		message = "插件重载超时"
	}

	var runtimeErr *pluginruntime.Error
	if errors.As(err, &runtimeErr) {
		if strings.TrimSpace(runtimeErr.Code) != "" {
			code = runtimeErr.Code
		}
		if strings.TrimSpace(runtimeErr.Message) != "" {
			message = runtimeErr.Message
		}
	} else {
		var specErr *pluginruntime.Error
		if errors.As(err, &specErr) {
			if strings.TrimSpace(specErr.Code) != "" {
				code = specErr.Code
			}
			if strings.TrimSpace(specErr.Message) != "" {
				message = specErr.Message
			}
			c.failReloadTask(taskID, pluginID, code, message)
			return
		}
		if strings.TrimSpace(err.Error()) != "" {
			message = err.Error()
		}
	}

	c.failReloadTask(taskID, pluginID, code, message)
}
