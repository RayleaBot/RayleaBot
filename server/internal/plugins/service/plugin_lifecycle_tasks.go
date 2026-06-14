package service

import (
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

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
