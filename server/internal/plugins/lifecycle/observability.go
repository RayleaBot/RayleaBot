package lifecycle

import (
	"context"
	"errors"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func (c *Controller) publishRuntimeState(pluginID, state string) {
	code, message := "", ""
	if state == string(pluginruntime.StateStopped) {
		if manager, ok := c.runtimes.Get(pluginID); ok && manager != nil {
			actual := manager.Snapshot()
			code, message = actual.LastErrorCode, actual.LastErrorMessage
			switch actual.State {
			case pluginruntime.StateStarting, pluginruntime.StateRunning, pluginruntime.StateStopping:
				state = string(actual.State)
			}
		}
	}
	if manager, ok := c.runtimes.Get(pluginID); ok {
		actual := manager.Snapshot()
		if actual.State == pluginruntime.State(state) {
			code, message = actual.LastErrorCode, actual.LastErrorMessage
		}
	}
	if _, err := c.plugins.SetRuntimeResult(pluginID, state, code, message); err != nil {
		c.logLifecycleWarn("publish plugin runtime state", pluginID, err)
	}
}

func (c *Controller) publishRuntimeFailure(pluginID string, cause error) {
	if cancellationOnly(cause) {
		c.publishRuntimeState(pluginID, string(pluginruntime.StateStopped))
		return
	}
	state := string(pluginruntime.StateStopped)
	if manager, ok := c.runtimes.Get(pluginID); ok && manager != nil {
		state = string(manager.Snapshot().State)
	}
	code := errorcodes.PluginInternalError
	var runtimeErr *plugins.Error
	if errors.As(cause, &runtimeErr) {
		code = runtimeErr.Code
	}
	definition, ok := errorcodes.Lookup(code)
	message := "插件初始化失败"
	if ok {
		message = definition.Message
	}
	if _, err := c.plugins.SetRuntimeResult(pluginID, state, code, message); err != nil {
		c.logLifecycleWarn("publish plugin initialization failure", pluginID, err)
	}
}

func (c *Controller) logLifecycleWarn(message, pluginID string, err error) {
	if c.logger == nil || err == nil || cancellationOnly(err) {
		return
	}

	pluginLabel, pluginName := c.pluginLogLabel(pluginID)
	c.logger.Warn(
		"插件生命周期操作失败", "plugin_label", pluginLabel, "operation", message, "operation_label", lifecycleActionLabel(message),
		"component", "app",
		"plugin_id", pluginID,
		"plugin_name", pluginName,
		"err", err.Error(),
	)
}

func (c *Controller) pluginLogLabel(pluginID string) (string, string) {
	pluginID = strings.TrimSpace(pluginID)
	if c != nil && c.plugins != nil {
		if snapshot, ok := c.plugins.Get(pluginID); ok {
			return plugins.DisplayLabel(snapshot), snapshot.Name
		}
	}
	if pluginID == "" {
		return "未知插件", ""
	}
	return pluginID, ""
}

func lifecycleActionLabel(message string) string {
	switch strings.TrimSpace(message) {
	case "start plugin runtime during reload":
		return "重新加载时启动"
	case "start stopped plugin runtime during reload":
		return "重新加载时启动"
	case "restart plugin runtime during reload":
		return "重新加载时重启"
	case "build runtime spec for plugin reload":
		return "准备启动配置"
	case "reload plugin runtime":
		return "重新加载"
	case "restart plugin after crash backoff":
		return "自动重启"
	case "stop plugin runtime":
		return "停止"
	case "plugin runtime reconcile failed":
		return "启动"
	case "start plugin runtime after enable":
		return "启用后启动"
	case "create plugin reload task":
		return "创建重新加载任务"
	default:
		if strings.TrimSpace(message) == "" {
			return "处理"
		}
		return "处理：" + strings.TrimSpace(message)
	}
}

func (c *Controller) projectRuntimeResult(pluginID string, manager *pluginruntime.Manager) {
	actual := manager.Snapshot()
	if _, err := c.plugins.SetRuntimeResult(pluginID, string(actual.State), actual.LastErrorCode, actual.LastErrorMessage); err != nil {
		c.logLifecycleWarn("publish plugin runtime state", pluginID, err)
	}
}

// A kill can return before OS reaping; keep the runtime registered and publish
// its final observation only if it still owns this plugin generation.
func (c *Controller) projectAfterPendingStop(pluginID string, manager *pluginruntime.Manager) {
	done := manager.ProcessDone()
	if done == nil {
		return
	}
	c.launch(func() {
		select {
		case <-done:
		case <-c.lifecycleContext().Done():
			return
		}
		release, err := c.acquireOperation(c.lifecycleContext(), pluginID)
		if err != nil {
			return
		}
		defer release()
		if current, ok := c.runtimes.Get(pluginID); !ok || current != manager {
			return
		}
		ctx, cancel := context.WithTimeout(context.WithoutCancel(c.lifecycleContext()), manager.ShutdownGrace())
		defer cancel()
		_ = manager.Stop(ctx)
		c.projectRuntimeResult(pluginID, manager)
	})
}
