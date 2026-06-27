package lifecycle

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func (c *Controller) logLifecycleWarn(message, pluginID string, err error) {
	if c == nil || c.logger == nil || err == nil {
		return
	}

	pluginLabel, pluginName := c.pluginLogLabel(pluginID)
	c.logger.Warn(
		"插件"+pluginLabel+lifecycleActionLabel(message)+"失败",
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
		return "重载时启动运行时"
	case "start stopped plugin runtime during reload":
		return "重载时启动已停止的运行时"
	case "restart plugin runtime during reload":
		return "重载时重启运行时"
	case "build runtime spec for plugin reload":
		return "重载时生成运行时配置"
	case "reload plugin runtime":
		return "重载运行时"
	case "restart plugin after crash backoff":
		return "崩溃等待后重启"
	case "stop plugin runtime":
		return "停止运行时"
	case "plugin runtime reconcile failed":
		return "对齐运行时状态"
	case "start plugin runtime after enable":
		return "启用后启动运行时"
	case "create plugin reload task":
		return "创建重载任务"
	default:
		if strings.TrimSpace(message) == "" {
			return "处理"
		}
		return "处理：" + strings.TrimSpace(message)
	}
}
