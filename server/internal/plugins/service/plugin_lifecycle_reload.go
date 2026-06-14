package service

import (
	"context"
	"errors"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/runtime/manager"
	runtimespec "github.com/RayleaBot/RayleaBot/server/internal/runtime/spec"
)

func (c *Controller) reloadPluginAsync(pluginID, botID string, taskID string) {
	if c == nil {
		return
	}
	botID = strings.TrimSpace(botID)
	c.startReloadTask(taskID)

	ctx, cancel := context.WithTimeout(context.Background(), runtimeInitTimeout(c.config().Runtime))
	defer cancel()

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok || snapshot.DesiredState != "enabled" {
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtimemanager.StateStopped))
		c.failReloadTask(taskID, pluginID, "platform.invalid_request", "插件当前不可重载")
		return
	}
	if _, err := c.validateActivation(ctx, snapshot); err != nil {
		c.disablePluginForPermissionLoss(ctx, pluginID)
		c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
		return
	}

	current, ok := c.runtimes.Get(pluginID)
	if !ok || current == nil {
		c.updateReloadTask(taskID, 30, "启动插件运行时")
		manager := c.runtimes.GetOrCreate(pluginID)
		if err := c.startRuntime(ctx, pluginID, botID, manager); err != nil {
			c.logLifecycleWarn("start plugin runtime during reload", pluginID, err)
			_, _ = c.plugins.SetRuntimeState(pluginID, string(runtimemanager.StateStopped))
			c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
			return
		}
		c.finishReloadTask(taskID, pluginID)
		return
	}

	switch current.Snapshot().State {
	case runtimemanager.StateStopped:
		c.updateReloadTask(taskID, 30, "启动插件运行时")
		if err := c.startRuntime(ctx, pluginID, botID, current); err != nil {
			c.logLifecycleWarn("start stopped plugin runtime during reload", pluginID, err)
			_, _ = c.plugins.SetRuntimeState(pluginID, string(runtimemanager.StateStopped))
			c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
			return
		}
		c.finishReloadTask(taskID, pluginID)
		return
	case runtimemanager.StateBackoff, runtimemanager.StateCrashed, runtimemanager.StateDeadLetter:
		current.ResetCrashCount()
		current.SetStopped()
		c.updateReloadTask(taskID, 30, "重置插件运行时")
		if err := c.startRuntime(ctx, pluginID, botID, current); err != nil {
			c.logLifecycleWarn("restart plugin runtime during reload", pluginID, err)
			_, _ = c.plugins.SetRuntimeState(pluginID, string(runtimemanager.StateStopped))
			c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
			return
		}
		c.finishReloadTask(taskID, pluginID)
		return
	case runtimemanager.StateStarting, runtimemanager.StateStopping:
		c.failReloadTask(taskID, pluginID, "platform.invalid_request", "插件运行时正在切换状态")
		return
	}

	c.updateReloadTask(taskID, 30, "构建插件运行时")
	spec, payload, err := c.buildStartInputs(ctx, pluginID, botID)
	if err != nil {
		c.logLifecycleWarn("build runtime spec for plugin reload", pluginID, err)
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtimemanager.StateStopped))
		c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
		return
	}

	newManager := c.runtimes.NewDetached()
	c.updateReloadTask(taskID, 60, "重载插件运行时")
	if err := c.dispatcher.ReloadPlugin(ctx, pluginID, current, newManager, spec, payload, dispatchCommands(snapshot.Commands)); err != nil {
		c.logLifecycleWarn("reload plugin runtime", pluginID, err)
		_, _ = c.plugins.SetRuntimeState(pluginID, string(runtimemanager.StateRunning))
		c.failReloadTaskForError(taskID, pluginID, err, "插件重载失败")
		return
	}

	c.runtimes.Replace(pluginID, newManager)
	newManager.ResetCrashCount()
	_, _ = c.plugins.SetRuntimeState(pluginID, string(runtimemanager.StateRunning))
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

	var runtimeErr *runtimemanager.Error
	if errors.As(err, &runtimeErr) {
		if strings.TrimSpace(runtimeErr.Code) != "" {
			code = runtimeErr.Code
		}
		if strings.TrimSpace(runtimeErr.Message) != "" {
			message = runtimeErr.Message
		}
	} else {
		var specErr *runtimespec.Error
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
		var pending *plugins.PermissionPendingError
		if errors.As(err, &pending) {
			code = "plugin.permission_pending"
			message = pending.Error()
		} else if strings.TrimSpace(err.Error()) != "" {
			message = err.Error()
		}
	}

	c.failReloadTask(taskID, pluginID, code, message)
}
