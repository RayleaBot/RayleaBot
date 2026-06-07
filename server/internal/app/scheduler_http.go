package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

type schedulerJobListResponse struct {
	Items []schedulerJobSummary `json:"items"`
}

type schedulerJobSummary struct {
	JobID          string                     `json:"job_id"`
	PluginID       string                     `json:"plugin_id"`
	PluginName     string                     `json:"plugin_name"`
	TaskName       string                     `json:"task_name"`
	LogLabel       string                     `json:"log_label"`
	CronExpr       string                     `json:"cron_expr"`
	Timezone       string                     `json:"timezone"`
	Enabled        bool                       `json:"enabled"`
	NextRun        string                     `json:"next_run"`
	LastRun        *string                    `json:"last_run"`
	LastDurationMS int64                      `json:"last_duration_ms"`
	LastError      *schedulerJobLastError     `json:"last_error,omitempty"`
	PayloadSummary schedulerJobPayloadSummary `json:"payload_summary"`
	Stats          schedulerJobRunStats       `json:"stats"`
}

type schedulerJobLastError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	At      string `json:"at"`
}

type schedulerJobPayloadSummary struct {
	ConversationID string `json:"conversation_id"`
	TargetType     string `json:"target_type"`
	TargetID       string `json:"target_id"`
	Content        string `json:"content"`
}

type schedulerJobRunStats struct {
	Total   int64 `json:"total"`
	Success int64 `json:"success"`
	Failed  int64 `json:"failed"`
	Timeout int64 `json:"timeout"`
	Retry   int64 `json:"retry"`
	Other   int64 `json:"other"`
}

type schedulerJobTriggerResponse struct {
	JobID     string `json:"job_id"`
	PluginID  string `json:"plugin_id"`
	Triggered bool   `json:"triggered"`
}

func (h *systemHTTPHandlers) handleSystemSchedulerJobList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.scheduler == nil {
			writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "scheduler_job",
			})
			return
		}

		jobs := h.scheduler.Jobs()
		sort.Slice(jobs, func(i, j int) bool {
			if jobs[i].PluginID == jobs[j].PluginID {
				return jobs[i].JobID < jobs[j].JobID
			}
			return jobs[i].PluginID < jobs[j].PluginID
		})
		items := make([]schedulerJobSummary, 0, len(jobs))
		for _, job := range jobs {
			items = append(items, h.schedulerJobSummary(job))
		}
		writeAuthJSON(w, http.StatusOK, schedulerJobListResponse{Items: items})
	}
}

func (h *systemHTTPHandlers) handleSystemSchedulerJobTrigger() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.scheduler == nil {
			writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "scheduler_job",
			})
			return
		}

		jobID := chi.URLParam(r, "job_id")
		job, err := h.scheduler.Trigger(context.WithoutCancel(r.Context()), jobID)
		if err != nil {
			if errors.Is(err, scheduler.ErrJobNotFound) {
				writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
					"resource_type": "scheduler_job",
					"job_id":        jobID,
				})
				return
			}
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		writeAuthJSON(w, http.StatusOK, schedulerJobTriggerResponse{
			JobID:     job.JobID,
			PluginID:  job.PluginID,
			Triggered: true,
		})
	}
}

func (h *systemHTTPHandlers) schedulerJobSummary(job scheduler.Job) schedulerJobSummary {
	pluginName := strings.TrimSpace(job.PluginID)
	if h != nil && h.system != nil && h.system.plugins != nil {
		if snapshot, ok := h.system.plugins.Get(job.PluginID); ok {
			pluginName = schedulerPluginDisplayName(snapshot, job.PluginID)
		}
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
		Timezone:       h.schedulerTimezone(),
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

func (h *systemHTTPHandlers) schedulerTimezone() string {
	if h != nil && h.system != nil && h.system.state != nil {
		if tz := strings.TrimSpace(h.system.state.Config.Scheduler.Timezone); tz != "" {
			return tz
		}
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
