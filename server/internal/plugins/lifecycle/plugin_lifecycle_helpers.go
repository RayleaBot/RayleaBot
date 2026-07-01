package lifecycle

import (
	"context"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func persistPluginDesiredState(ctx context.Context, repo plugins.DesiredStateRepository, pluginID, desiredState string) error {
	if repo == nil {
		return nil
	}
	return repo.SaveDesiredState(ctx, pluginID, desiredState, time.Now().UTC())
}

func dispatchCommands(commands []plugins.Command) []dispatch.CommandDecl {
	items := make([]dispatch.CommandDecl, 0, len(commands))
	for _, command := range commands {
		if strings.TrimSpace(command.Name) == "" {
			continue
		}
		items = append(items, dispatch.CommandDecl{
			Name:         command.Name,
			Aliases:      append([]string(nil), command.Aliases...),
			MatchPattern: command.MatchPattern,
			Permission:   command.Permission,
		})
	}
	return items
}

func runtimeInitTimeout(cfg config.RuntimeConfig) time.Duration {
	seconds := cfg.PluginInitMaxTotalSeconds
	if seconds <= 0 {
		seconds = 300
	}
	return time.Duration(seconds+5) * time.Second
}

func (c *Controller) seedPluginDefaultConfig(ctx context.Context, snapshot plugins.Snapshot) error {
	if c == nil || c.pluginConfig == nil || len(snapshot.DefaultConfig) == 0 {
		return nil
	}
	_, err := c.pluginConfig.SeedDefaults(ctx, snapshot.PluginID, snapshot.DefaultConfig)
	return err
}

func (c *Controller) reconcileRecoverySummaryBestEffort(trigger string) {
	if c == nil || c.onRecoveryChange == nil {
		return
	}
	c.onRecoveryChange(trigger)
}

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

func (c *Controller) createReloadTask(pluginID string, snapshot plugins.Snapshot) string {
	if c == nil || c.tasks == nil {
		return ""
	}
	displayName := strings.TrimSpace(snapshot.Name)
	if displayName == "" {
		displayName = pluginID
	}
	taskID, err := c.tasks.Create("plugin.reload", "reload plugin: "+displayName)
	if err != nil {
		c.logLifecycleWarn("create plugin reload task", pluginID, err)
		return ""
	}
	return taskID
}

func (c *Controller) startReloadTask(taskID string) {
	if c == nil || c.tasks == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	now := time.Now().UTC()
	c.tasks.Update(taskID, tasks.Update{
		Status:    lifecycleTaskStatusPtr(tasks.StatusRunning),
		Progress:  lifecycleIntPtr(5),
		Summary:   lifecycleStringPtr("准备重载插件"),
		StartedAt: &now,
	})
}

func (c *Controller) updateReloadTask(taskID string, progress int, summary string) {
	if c == nil || c.tasks == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	c.tasks.Update(taskID, tasks.Update{
		Progress: lifecycleIntPtr(progress),
		Summary:  lifecycleStringPtr(summary),
	})
}

func (c *Controller) finishReloadTask(taskID string, pluginID string) {
	if c == nil || c.tasks == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	now := time.Now().UTC()
	c.tasks.Update(taskID, tasks.Update{
		Status:     lifecycleTaskStatusPtr(tasks.StatusSucceeded),
		Progress:   lifecycleIntPtr(100),
		Summary:    lifecycleStringPtr("插件重载完成"),
		FinishedAt: &now,
		Result: &tasks.ResultSummary{
			Summary: "插件运行时已重载",
			Details: map[string]any{
				"plugin_id": pluginID,
			},
		},
	})
}

func (c *Controller) failReloadTask(taskID string, pluginID string, code string, message string) {
	if c == nil || c.tasks == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	if strings.TrimSpace(code) == "" {
		code = "plugin.internal_error"
	}
	if strings.TrimSpace(message) == "" {
		message = "插件重载失败"
	}
	now := time.Now().UTC()
	c.tasks.Update(taskID, tasks.Update{
		Status:     lifecycleTaskStatusPtr(tasks.StatusFailed),
		Summary:    lifecycleStringPtr(message),
		FinishedAt: &now,
		Error: &tasks.ErrorSummary{
			Code:    code,
			Message: message,
			Details: map[string]any{
				"plugin_id": pluginID,
			},
		},
	})
}

func lifecycleStringPtr(value string) *string {
	return &value
}

func lifecycleIntPtr(value int) *int {
	return &value
}

func lifecycleTaskStatusPtr(status tasks.Status) *tasks.Status {
	return &status
}
