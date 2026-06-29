package tasks

import (
	"fmt"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

type taskLogEvent string

const (
	taskLogEventCreated       taskLogEvent = "created"
	taskLogEventStatusChanged taskLogEvent = "status_changed"
)

func appendTaskLog(logs LogSink, snapshot Snapshot, event taskLogEvent) {
	if logs == nil {
		return
	}

	details := taskLogDetails(snapshot, event)
	logs.Append(logging.Summary{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     taskLogLevel(snapshot.Status),
		Source:    "tasks",
		Message:   taskLogMessage(snapshot),
		PluginID:  readTaskDetailString(details, "plugin_id"),
		Protocol:  readTaskDetailString(details, "protocol"),
		RequestID: readTaskDetailString(details, "request_id"),
		Details:   details,
	})
}

func taskLogLevel(status Status) string {
	switch status {
	case StatusFailed:
		return "error"
	case StatusCancelled, StatusInterrupted:
		return "warn"
	default:
		return "info"
	}
}

func taskLogMessage(snapshot Snapshot) string {
	statusText := map[Status]string{
		StatusPending:     "任务已提交",
		StatusRunning:     "任务执行中",
		StatusSucceeded:   "任务完成",
		StatusFailed:      "任务失败",
		StatusCancelled:   "任务已取消",
		StatusInterrupted: "任务已中断",
	}[snapshot.Status]
	if statusText == "" {
		statusText = "任务状态更新"
	}

	summary := strings.TrimSpace(snapshot.Summary)
	if summary == "" {
		return fmt.Sprintf("%s %s", statusText, snapshot.TaskType)
	}
	return fmt.Sprintf("%s %s：%s", statusText, snapshot.TaskType, summary)
}

func taskLogDetails(snapshot Snapshot, event taskLogEvent) map[string]any {
	details := map[string]any{
		"task_event":   string(event),
		"task_id":      snapshot.TaskID,
		"task_type":    snapshot.TaskType,
		"task_status":  string(snapshot.Status),
		"task_summary": snapshot.Summary,
	}
	if snapshot.Progress > 0 {
		details["task_progress"] = snapshot.Progress
	}
	if snapshot.StartedAt != nil {
		details["started_at"] = snapshot.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	if snapshot.FinishedAt != nil {
		details["finished_at"] = snapshot.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	if snapshot.Result != nil {
		details["result_summary"] = snapshot.Result.Summary
		if len(snapshot.Result.Details) > 0 {
			details["result_details"] = snapshot.Result.Details
		}
		mergeTaskContext(details, snapshot.Result.Details)
	}
	if snapshot.Error != nil {
		details["error_code"] = snapshot.Error.Code
		details["error_message"] = snapshot.Error.Message
		if len(snapshot.Error.Details) > 0 {
			details["error_details"] = snapshot.Error.Details
		}
		mergeTaskContext(details, snapshot.Error.Details)
	}
	return details
}

func mergeTaskContext(target map[string]any, source map[string]any) {
	for _, key := range []string{"plugin_id", "protocol", "request_id"} {
		if value := readTaskDetailString(source, key); value != "" {
			target[key] = value
		}
	}
}

func readTaskDetailString(source map[string]any, key string) string {
	if len(source) == 0 {
		return ""
	}
	value, _ := source[key].(string)
	return strings.TrimSpace(value)
}
