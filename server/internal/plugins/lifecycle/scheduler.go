package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

func (c *Controller) HandleSchedulerTrigger(ctx context.Context, job scheduler.Job) {

	pluginID := strings.TrimSpace(job.PluginID)
	if pluginID == "" {
		return
	}
	taskName := strings.TrimSpace(job.JobID)
	logLabel := scheduler.DisplayLabel(job.LogLabel)
	startedAt := time.Now()

	snapshot, ok := c.plugins.Get(pluginID)
	if ok && snapshot.DesiredState != "enabled" {
		c.recordSchedulerRunResult(ctx, taskName, job.Revision, scheduler.RunOutcomeOther, time.Since(startedAt), errorcodes.PluginEventCanceled, "插件已停用，本轮未执行", time.Now())
		return
	}
	if !ok || snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" || !snapshot.Valid {
		c.logSchedulerTriggerFailure(ctx, pluginID, schedulerPluginDisplayName(snapshot, pluginID), taskName, logLabel, job.Revision, startedAt, errorcodes.PlatformInvalidRequest, "plugin is not available")
		return
	}

	if err := c.ensurePluginRunning(ctx, pluginID); err != nil {
		c.logSchedulerTriggerFailure(ctx, pluginID, schedulerPluginDisplayName(snapshot, pluginID), taskName, logLabel, job.Revision, startedAt, errorcodes.PluginInternalError, err.Error())
		return
	}

	pluginName := schedulerPluginDisplayName(snapshot, pluginID)

	result := c.dispatcher.DispatchScheduledEvent(ctx, pluginID, chatevent.Event{
		EventID:        fmt.Sprintf("scheduler-%s-%d", job.JobID, time.Now().UnixNano()),
		SourceProtocol: "scheduler",
		SourceAdapter:  "scheduler.internal",
		EventType:      "scheduler.trigger",
		Timestamp:      startedAt.Unix(),
		PayloadFields:  schedulerPayloadFields(job),
	}, scheduler.RunContext{
		JobID:      job.JobID,
		Revision:   job.Revision,
		PluginName: pluginName,
		TaskName:   taskName,
		LogLabel:   logLabel,
		StartedAt:  startedAt,
		Recorder:   c.scheduler,
	})
	if result.Outcome != dispatch.OutcomeDelivered {
		c.logSchedulerTriggerFailure(ctx, pluginID, pluginName, taskName, logLabel, job.Revision, startedAt, result.ErrorCode, string(result.Outcome))
	} else if count := c.schedulerFailures.Recover(pluginID + ":" + taskName); count > 0 && c.logger != nil {
		c.logger.Info("定时任务已恢复，等待执行", "display_summary", scheduler.DisplayMessage(pluginName, taskName, logLabel, "已恢复，等待执行"), "component", "scheduler", "plugin_id", pluginID, "job_id", taskName, "repeat_count", count)
	}
}

func (c *Controller) logSchedulerTriggerFailure(ctx context.Context, pluginID, pluginName, taskName, logLabel string, revision uint64, startedAt time.Time, errorCode, errorText string) {
	duration := time.Since(startedAt)
	outcome := scheduler.RunOutcomeFailed
	if errorCode == errorcodes.PluginEventCanceled || ctx.Err() == context.Canceled {
		outcome, errorCode, errorText = scheduler.RunOutcomeOther, errorcodes.PluginEventCanceled, "本轮调度已取消"
	}
	c.recordSchedulerRunResult(ctx, taskName, revision, outcome, duration, errorCode, errorText, time.Now())
	if c.logger == nil {
		return
	}
	if outcome == scheduler.RunOutcomeOther {
		return
	}
	count := c.schedulerFailures.Failure(pluginID+":"+taskName, errorCode, time.Now())
	if count == 0 {
		return
	}
	message := scheduler.DisplayMessage(pluginName, taskName, logLabel, "未执行") + "；插件暂时无法接收任务，请检查插件状态。"
	if count > 1 {
		message += fmt.Sprintf("（期间重复 %d 次）", count)
	}
	c.logger.Warn(
		"定时任务未执行，插件暂时无法接收任务",
		"display_summary", message,
		"component", "scheduler",
		"plugin_id", pluginID,
		"plugin_name", pluginName,
		"job_id", taskName,
		"log_label", logLabel,
		"duration_ms", duration.Milliseconds(),
		"error_code", errorCode,
		"error", errorText,
		"repeat_count", count,
	)
}

func (c *Controller) recordSchedulerRunResult(ctx context.Context, jobID string, revision uint64, outcome scheduler.RunOutcome, duration time.Duration, errorCode, errorText string, occurredAt time.Time) {
	if c.scheduler == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := c.scheduler.RecordRunResult(ctx, scheduler.RunResult{
		JobID:      jobID,
		Revision:   revision,
		Outcome:    outcome,
		Duration:   duration,
		ErrorCode:  errorCode,
		ErrorText:  errorText,
		OccurredAt: occurredAt,
	}); err != nil && c.logger != nil {
		c.logger.Warn(
			"定时任务结果保存失败，历史记录可能缺失",
			"component", "scheduler",
			"job_id", jobID,
			"err", err.Error(),
		)
	}
}

func schedulerPluginDisplayName(snapshot plugins.Snapshot, pluginID string) string {
	if name := strings.TrimSpace(snapshot.Name); name != "" {
		return name
	}
	if pluginID = strings.TrimSpace(pluginID); pluginID != "" {
		return pluginID
	}
	return "未知插件"
}

func schedulerPayloadFields(job scheduler.Job) map[string]any {
	fields := make(map[string]any, 2)
	if len(job.Payload) == 0 || string(job.Payload) == "null" {
		return fields
	}
	var payload map[string]any
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fields
	}
	fields["payload"] = payload
	if action, ok := payload["action"].(string); ok && strings.TrimSpace(action) != "" {
		fields["action"] = action
	}
	return fields
}
