package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

func (c *Controller) HandleSchedulerTrigger(ctx context.Context, job scheduler.Job) {
	if c == nil {
		return
	}

	pluginID := strings.TrimSpace(job.PluginID)
	if pluginID == "" {
		return
	}
	taskName := strings.TrimSpace(job.JobID)
	logLabel := scheduler.DisplayLabel(job.LogLabel)
	startedAt := time.Now()

	snapshot, ok := c.plugins.Get(pluginID)
	if !ok || snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" || !snapshot.Valid {
		c.logSchedulerTriggerFailure(ctx, pluginID, schedulerPluginDisplayName(snapshot, pluginID), taskName, logLabel, startedAt, "platform.invalid_request", "plugin is not available")
		return
	}

	if err := c.ensurePluginRunning(ctx, pluginID, c.currentBotID()); err != nil {
		c.logSchedulerTriggerFailure(ctx, pluginID, schedulerPluginDisplayName(snapshot, pluginID), taskName, logLabel, startedAt, "plugin.internal_error", err.Error())
		return
	}

	pluginName := schedulerPluginDisplayName(snapshot, pluginID)

	result := c.dispatcher.DispatchToPlugin(ctx, pluginID, runtimeprotocol.Event{
		EventID:        fmt.Sprintf("scheduler-%s-%d", job.JobID, time.Now().UnixNano()),
		SourceProtocol: "scheduler",
		SourceAdapter:  "scheduler.internal",
		EventType:      "scheduler.trigger",
		Timestamp:      startedAt.Unix(),
		PayloadFields:  schedulerPayloadFields(job),
		SchedulerLog: &runtimeprotocol.SchedulerLogContext{
			JobID:      job.JobID,
			PluginName: pluginName,
			TaskName:   taskName,
			LogLabel:   logLabel,
			StartedAt:  startedAt,
			Recorder:   c.scheduler,
		},
	})
	if result.Outcome != dispatch.OutcomeDelivered {
		c.logSchedulerTriggerFailure(ctx, pluginID, pluginName, taskName, logLabel, startedAt, result.ErrorCode, string(result.Outcome))
	}
}

func (c *Controller) logSchedulerTriggerFailure(ctx context.Context, pluginID, pluginName, taskName, logLabel string, startedAt time.Time, errorCode, errorText string) {
	if c == nil {
		return
	}
	duration := time.Since(startedAt)
	c.recordSchedulerRunResult(ctx, taskName, scheduler.RunOutcomeFailed, duration, errorCode, errorText, time.Now())
	if c.logger == nil {
		return
	}
	c.logger.Warn(
		scheduler.DisplayMessage(pluginName, taskName, logLabel, "处理失败")+"耗时 "+scheduler.FormatDuration(duration),
		"component", "scheduler",
		"plugin_id", pluginID,
		"plugin_name", pluginName,
		"job_id", taskName,
		"log_label", logLabel,
		"duration_ms", duration.Milliseconds(),
		"error_code", errorCode,
		"error", errorText,
	)
}

func (c *Controller) recordSchedulerRunResult(ctx context.Context, jobID string, outcome scheduler.RunOutcome, duration time.Duration, errorCode, errorText string, occurredAt time.Time) {
	if c == nil || c.scheduler == nil {
		return
	}
	if err := c.scheduler.RecordRunResult(ctx, scheduler.RunResult{
		JobID:      jobID,
		Outcome:    outcome,
		Duration:   duration,
		ErrorCode:  errorCode,
		ErrorText:  errorText,
		OccurredAt: occurredAt,
	}); err != nil && c.logger != nil {
		c.logger.Warn(
			"scheduler run state update failed",
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

func SchedulerPluginDisplayName(snapshot plugins.Snapshot, pluginID string) string {
	return schedulerPluginDisplayName(snapshot, pluginID)
}

func schedulerPayloadFields(job scheduler.Job) map[string]any {
	fields := map[string]any{
		"job_id": job.JobID,
	}
	if len(job.Payload) == 0 || string(job.Payload) == "null" {
		return fields
	}
	var payload map[string]any
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fields
	}
	for key, value := range payload {
		fields[key] = value
	}
	return fields
}
