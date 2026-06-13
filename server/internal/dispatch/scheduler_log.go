package dispatch

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

func schedulerElapsed(event runtime.Event) time.Duration {
	if event.SchedulerLog == nil {
		return 0
	}
	return time.Since(event.SchedulerLog.StartedAt)
}

func (d *Dispatcher) logSchedulerCompletion(pluginID string, event runtime.Event, status string, duration time.Duration, extra map[string]any) {
	if d == nil || d.logger == nil || event.SchedulerLog == nil {
		return
	}
	ctx := event.SchedulerLog
	attrs := []any{
		"component", "scheduler",
		"plugin_id", pluginID,
		"plugin_name", ctx.PluginName,
		"job_id", ctx.TaskName,
		"log_label", ctx.LogLabel,
		"duration_ms", duration.Milliseconds(),
	}
	for key, value := range extra {
		attrs = append(attrs, key, value)
	}
	message := schedulerCompletionMessage(ctx.PluginName, ctx.TaskName, ctx.LogLabel, status, duration)
	if status == "处理失败" {
		d.logger.Warn(message, attrs...)
		return
	}
	d.logger.Info(message, attrs...)
}

func (d *Dispatcher) recordSchedulerCompletion(ctx context.Context, event runtime.Event, outcome scheduler.RunOutcome, duration time.Duration, errorCode, errorText string) {
	if event.SchedulerLog == nil || event.SchedulerLog.Recorder == nil {
		return
	}
	jobID := strings.TrimSpace(event.SchedulerLog.JobID)
	if jobID == "" {
		jobID = strings.TrimSpace(event.SchedulerLog.TaskName)
	}
	if jobID == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := event.SchedulerLog.Recorder.RecordSchedulerRunResult(ctx, runtime.SchedulerRunResult{
		JobID:      jobID,
		Outcome:    string(outcome),
		Duration:   duration,
		ErrorCode:  errorCode,
		ErrorText:  errorText,
		OccurredAt: time.Now(),
	}); err != nil && d.logger != nil {
		d.logger.Warn(
			"scheduler run state update failed",
			"component", "scheduler",
			"job_id", jobID,
			"err", err.Error(),
		)
	}
}

func schedulerFailureFields(err error, delivery runtime.Delivery) (scheduler.RunOutcome, string, string) {
	code := strings.TrimSpace(delivery.ErrorCode)
	message := strings.TrimSpace(delivery.ErrorMessage)
	if code == "" {
		var runtimeErr *runtime.Error
		if errors.As(err, &runtimeErr) {
			code = runtimeErr.Code
			message = runtimeErr.Message
		}
	}
	if message == "" && err != nil {
		message = err.Error()
	}
	if strings.Contains(strings.ToLower(code), "timeout") {
		return scheduler.RunOutcomeTimeout, code, message
	}
	return scheduler.RunOutcomeFailed, code, message
}

func schedulerCompletionMessage(pluginName, taskName, logLabel, status string, duration time.Duration) string {
	return scheduler.DisplayMessage(pluginName, taskName, logLabel, status) + "耗时 " + scheduler.FormatDuration(duration)
}
