package managementhttp

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

func (s *schedulerHTTPServiceImpl) schedulerJobSummary(job scheduler.Job) schedulerJobSummary {
	pluginName := strings.TrimSpace(job.PluginID)
	if s != nil && s.system != nil {
		pluginName = s.system.SchedulerPluginName(job.PluginID)
	}
	if pluginName == "" {
		pluginName = "未知插件"
	}
	lastRun := formatOptionalTime(job.LastRun)
	var lastError *schedulerJobLastError
	if job.LastError != nil && (job.LastError.Code != "" || job.LastError.Message != "") {
		at := job.LastError.At
		if at.IsZero() {
			at = time.Now().UTC()
		}
		lastError = &schedulerJobLastError{
			Code:    job.LastError.Code,
			Message: job.LastError.Message,
			At:      at.UTC().Format(time.RFC3339),
		}
	}
	return schedulerJobSummary{
		JobID:          job.JobID,
		PluginID:       job.PluginID,
		PluginName:     pluginName,
		TaskName:       scheduler.DisplayLabel(job.JobID, "未命名任务"),
		LogLabel:       scheduler.DisplayLabel(job.LogLabel),
		CronExpr:       job.CronExpr,
		Timezone:       s.SchedulerTimezone(),
		Enabled:        job.Enabled,
		NextRun:        job.NextRun.UTC().Format(time.RFC3339),
		LastRun:        lastRun,
		LastDurationMS: job.LastDurationMS,
		LastError:      lastError,
		PayloadSummary: summarizeSchedulerPayload(job.Payload),
		Stats: schedulerJobRunStats{
			Total:   job.RunStats.Total(),
			Success: job.RunStats.Success,
			Failed:  job.RunStats.Failed,
			Timeout: job.RunStats.Timeout,
			Retry:   job.RunStats.Retry,
			Other:   job.RunStats.Other,
		},
	}
}

func (s *schedulerHTTPServiceImpl) SchedulerTimezone() string {
	if s != nil && s.system != nil {
		return s.system.SchedulerTimezone()
	}
	return "UTC"
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func summarizeSchedulerPayload(raw json.RawMessage) schedulerJobPayloadSummary {
	var payload map[string]any
	if len(raw) == 0 || string(raw) == "null" {
		return schedulerJobPayloadSummary{}
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return schedulerJobPayloadSummary{}
	}
	targetType := firstPayloadText(payload, "target_type", "type")
	targetID := firstPayloadText(payload, "target_id", "group_id", "user_id", "conversation_id")
	conversationID := firstPayloadText(payload, "conversation_id", "session_id")
	if conversationID == "" && targetType != "" && targetID != "" {
		conversationID = targetType + ":" + targetID
	}
	return schedulerJobPayloadSummary{
		ConversationID: conversationID,
		TargetType:     targetType,
		TargetID:       targetID,
		Content:        firstPayloadText(payload, "content", "summary", "title", "topic", "message"),
	}
}

func firstPayloadText(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		if text := strings.TrimSpace(toSchedulerPayloadText(value)); text != "" {
			return text
		}
	}
	return ""
}

func toSchedulerPayloadText(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strings.TrimRight(strings.TrimRight(strconvFormatFloat(typed), "0"), ".")
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func strconvFormatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
