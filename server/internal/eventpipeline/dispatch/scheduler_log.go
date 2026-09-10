package dispatch

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

func schedulerElapsed(run *scheduler.RunContext) time.Duration {
	if run == nil {
		return 0
	}
	return time.Since(run.StartedAt)
}

func (d *Dispatcher) logSchedulerFailure(pluginID string, run *scheduler.RunContext, duration time.Duration, extra map[string]any) {
	if d.logger == nil || run == nil {
		return
	}
	attrs := []any{
		"component", "scheduler", "plugin_id", pluginID,
		"plugin_name", run.PluginName, "job_id", run.TaskName,
		"log_label", run.LogLabel, "duration_ms", duration.Milliseconds(),
	}
	for key, value := range extra {
		attrs = append(attrs, key, value)
	}
	code, _ := extra["error_code"].(string)
	if code == errorcodes.PluginEventCanceled {
		d.logger.Debug("定时任务已取消", attrs...)
		return
	}
	count := d.failures.Failure("scheduler:"+pluginID+":"+run.TaskName, code, time.Now())
	if count == 0 {
		return
	}
	attrs = append(attrs, "repeat_count", count, "failure_reason", eventFailureDescription(code))
	d.logger.Warn("定时任务处理失败", attrs...)
}

func eventFailureDescription(code string) string {
	switch code {
	case errorcodes.PluginEventTimeout:
		return "处理超时。"
	case errorcodes.PluginEventCanceled:
		return "任务已取消。"
	case errorcodes.PluginProtocolViolation:
		return "插件通信异常，已停止插件，请检查插件版本。"
	case errorcodes.PlatformInvalidRequest:
		return "插件暂不可用，本次未执行。"
	default:
		return "任务未完成，请查看错误详情。"
	}
}

func (d *Dispatcher) recoverScheduler(pluginID string, run *scheduler.RunContext) {
	if run == nil {
		return
	}
	job := run.TaskName
	if count := d.failures.Recover("scheduler:" + pluginID + ":" + job); count > 0 {
		d.logger.Info("定时任务已恢复", "display_summary", scheduler.DisplayMessage(run.PluginName, job, run.LogLabel, "已恢复"), "component", "scheduler", "plugin_id", pluginID, "job_id", job, "repeat_count", count)
	}
}

func (d *Dispatcher) recordSchedulerCompletion(ctx context.Context, run *scheduler.RunContext, outcome scheduler.RunOutcome, duration time.Duration, errorCode, errorText string) {
	if run == nil || run.Recorder == nil {
		return
	}
	jobID := strings.TrimSpace(run.JobID)
	if jobID == "" {
		jobID = strings.TrimSpace(run.TaskName)
	}
	if jobID == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := run.Recorder.RecordRunResult(ctx, scheduler.RunResult{
		JobID:      jobID,
		Revision:   run.Revision,
		Outcome:    outcome,
		Duration:   duration,
		ErrorCode:  errorCode,
		ErrorText:  errorText,
		OccurredAt: time.Now(),
	}); err != nil && d.logger != nil {
		d.logger.Warn(
			"定时任务结果保存失败，历史记录可能缺失",
			"component", "scheduler",
			"job_id", jobID,
			"err", err.Error(),
		)
	}
}

func schedulerFailureFields(err error, delivery plugins.Delivery) (scheduler.RunOutcome, string, string) {
	code := strings.TrimSpace(delivery.ErrorCode)
	message := strings.TrimSpace(delivery.ErrorMessage)
	if code == "" {
		var runtimeErr *plugins.Error
		if errors.As(err, &runtimeErr) && runtimeErr != nil {
			code = runtimeErr.Code
			message = runtimeErr.Message
		}
	}
	if message == "" && err != nil {
		message = err.Error()
	}
	if code == errorcodes.PluginEventCanceled || errors.Is(err, context.Canceled) {
		return scheduler.RunOutcomeOther, errorcodes.PluginEventCanceled, "事件因请求取消或运行时停止而结束"
	}
	if strings.Contains(strings.ToLower(code), "timeout") {
		return scheduler.RunOutcomeTimeout, code, message
	}
	return scheduler.RunOutcomeFailed, code, message
}
