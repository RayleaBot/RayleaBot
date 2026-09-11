package lifecycle

import (
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func (c *Controller) createReloadTask(pluginID string, snapshot plugins.Snapshot) string {
	if c.tasks == nil {
		return ""
	}
	displayName := strings.TrimSpace(snapshot.Name)
	if displayName == "" {
		displayName = pluginID
	}
	taskID, err := c.tasks.Create("plugin.reload", "重新加载插件“"+displayName+"”")
	if err != nil {
		c.logLifecycleWarn("create plugin reload task", pluginID, err)
		return ""
	}
	return taskID
}

func (c *Controller) startReloadTask(taskID string) {
	if c.tasks == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	now := time.Now().UTC()
	c.tasks.Update(taskID, tasks.Update{
		Status:    taskStatusPtr(tasks.StatusRunning),
		Progress:  intPtr(5),
		StartedAt: &now,
	})
}

func (c *Controller) updateReloadTask(taskID string, progress int, summary string) {
	if c.tasks == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	c.tasks.Update(taskID, tasks.Update{
		Progress: intPtr(progress),
		Summary:  stringPtr(summary),
	})
}

func (c *Controller) finishReloadTask(taskID string, pluginID string) {
	if c.tasks == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	now := time.Now().UTC()
	c.tasks.Update(taskID, tasks.Update{
		Status:     taskStatusPtr(tasks.StatusSucceeded),
		Progress:   intPtr(100),
		Summary:    stringPtr("插件“" + pluginID + "”已重新加载"),
		FinishedAt: &now,
		Result: &tasks.ResultSummary{
			Summary: "插件已重新加载",
			Details: map[string]any{
				"plugin_id": pluginID,
			},
		},
	})
}

func (c *Controller) failReloadTask(taskID string, pluginID string, code string, message string, details ...map[string]any) {
	if c.tasks == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	if strings.TrimSpace(code) == "" {
		code = errorcodes.PluginInternalError
	}
	if strings.TrimSpace(message) == "" {
		message = "插件重载失败"
	}
	now := time.Now().UTC()
	errorDetails := map[string]any{"plugin_id": pluginID}
	if len(details) > 0 {
		errorDetails = details[0]
	}
	c.tasks.Update(taskID, tasks.Update{
		Status:     taskStatusPtr(tasks.StatusFailed),
		Summary:    stringPtr(message),
		FinishedAt: &now,
		Error: &tasks.ErrorSummary{
			Code:    code,
			Message: message,
			Details: errorDetails,
		},
	})
}
