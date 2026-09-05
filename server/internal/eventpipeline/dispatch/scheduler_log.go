package dispatch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

func schedulerElapsed(event pluginruntime.Event) time.Duration {
	if event.SchedulerLog == nil {
		return 0
	}
	return time.Since(event.SchedulerLog.StartedAt)
}

func (d *Dispatcher) logSchedulerCompletion(pluginID string, event pluginruntime.Event, status string, duration time.Duration, extra map[string]any) {
	if d.logger == nil || event.SchedulerLog == nil {
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
	message := scheduler.DisplayMessage(ctx.PluginName, ctx.TaskName, ctx.LogLabel, status)
	if status == "处理失败" {
		code, _ := extra["error_code"].(string)
		if code == "plugin.event_canceled" {
			d.logger.Debug("定时任务已取消。", attrs...)
			return
		}
		count := d.failures.Failure("scheduler:"+pluginID+":"+ctx.TaskName, code, time.Now())
		if count == 0 {
			return
		}
		attrs = append(attrs, "repeat_count", count)
		message += "；" + eventFailureDescription(code)
		if count > 1 {
			message += fmt.Sprintf("；期间重复 %d 次。", count)
		}
		d.logger.Warn(message, attrs...)
		return
	}
	d.logger.Info(message, attrs...)
}

func eventFailureDescription(code string) string {
	switch code {
	case "plugin.event_timeout":
		return "处理超时。"
	case "plugin.event_canceled":
		return "任务已取消。"
	case "plugin.protocol_violation":
		return "插件通信异常，已停止插件，请检查插件版本。"
	case "platform.invalid_request":
		return "插件暂不可用，本次未执行。"
	default:
		return "任务未完成，请查看错误详情。"
	}
}

func (d *Dispatcher) recoverScheduler(pluginID string, event pluginruntime.Event) {
	if event.SchedulerLog == nil {
		return
	}
	job := event.SchedulerLog.TaskName
	if count := d.failures.Recover("scheduler:" + pluginID + ":" + job); count > 0 {
		d.logger.Info(scheduler.DisplayMessage(event.SchedulerLog.PluginName, job, event.SchedulerLog.LogLabel, "已恢复"), "component", "scheduler", "plugin_id", pluginID, "job_id", job, "repeat_count", count)
	}
}

func (d *Dispatcher) recordSchedulerCompletion(ctx context.Context, event pluginruntime.Event, outcome scheduler.RunOutcome, duration time.Duration, errorCode, errorText string) {
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
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := event.SchedulerLog.Recorder.RecordSchedulerRunResult(ctx, pluginruntime.SchedulerRunResult{
		JobID:      jobID,
		Revision:   event.SchedulerLog.Revision,
		Outcome:    string(outcome),
		Duration:   duration,
		ErrorCode:  errorCode,
		ErrorText:  errorText,
		OccurredAt: time.Now(),
	}); err != nil && d.logger != nil {
		d.logger.Warn(
			"定时任务 "+jobID+" 的结果保存失败，历史记录可能缺失："+err.Error(),
			"component", "scheduler",
			"job_id", jobID,
			"err", err.Error(),
		)
	}
}

func schedulerFailureFields(err error, delivery pluginruntime.Delivery) (scheduler.RunOutcome, string, string) {
	code := strings.TrimSpace(delivery.ErrorCode)
	message := strings.TrimSpace(delivery.ErrorMessage)
	if code == "" {
		var runtimeErr *pluginruntime.Error
		if errors.As(err, &runtimeErr) && runtimeErr != nil {
			code = runtimeErr.Code
			message = runtimeErr.Message
		}
	}
	if message == "" && err != nil {
		message = err.Error()
	}
	if code == "plugin.event_canceled" || errors.Is(err, context.Canceled) {
		return scheduler.RunOutcomeOther, "plugin.event_canceled", "事件因请求取消或运行时停止而结束"
	}
	if strings.Contains(strings.ToLower(code), "timeout") {
		return scheduler.RunOutcomeTimeout, code, message
	}
	return scheduler.RunOutcomeFailed, code, message
}
