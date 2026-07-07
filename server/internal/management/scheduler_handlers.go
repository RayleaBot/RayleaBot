package management

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/go-chi/chi/v5"
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

type schedulerHTTPServiceImpl struct {
	system    SchedulerMetadataService
	scheduler SchedulerEngineService
}

func newSchedulerHTTPService(system SchedulerMetadataService, scheduler SchedulerEngineService) *schedulerHTTPServiceImpl {
	return &schedulerHTTPServiceImpl{system: system, scheduler: scheduler}
}

func (h *SystemHandlers) HandleSystemSchedulerJobList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.scheduler == nil {
			WriteSystemHTTPError(w, r, missingSchedulerJobHTTPError(""))
			return
		}

		response, ok := h.scheduler.ListJobs()
		if !ok {
			WriteSystemHTTPError(w, r, missingSchedulerJobHTTPError(""))
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, response)
	}
}

func (s *schedulerHTTPServiceImpl) ListJobs() (schedulerJobListResponse, bool) {
	if s.scheduler == nil {
		return schedulerJobListResponse{}, false
	}
	jobs := s.scheduler.Jobs()
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].PluginID == jobs[j].PluginID {
			return jobs[i].JobID < jobs[j].JobID
		}
		return jobs[i].PluginID < jobs[j].PluginID
	})
	items := make([]schedulerJobSummary, 0, len(jobs))
	for _, job := range jobs {
		items = append(items, s.schedulerJobSummary(job))
	}
	return schedulerJobListResponse{Items: items}, true
}

func (s *schedulerHTTPServiceImpl) TriggerJob(ctx context.Context, jobID string) (schedulerJobTriggerResponse, *SystemHTTPError) {
	if s.scheduler == nil {
		return schedulerJobTriggerResponse{}, missingSchedulerJobHTTPError("")
	}
	job, err := s.scheduler.Trigger(ctx, jobID)
	if err != nil {
		if errors.Is(err, scheduler.ErrJobNotFound) {
			return schedulerJobTriggerResponse{}, missingSchedulerJobHTTPError(jobID)
		}
		return schedulerJobTriggerResponse{}, InternalSystemHTTPError()
	}
	return schedulerJobTriggerResponse{
		JobID:     job.JobID,
		PluginID:  job.PluginID,
		Triggered: true,
	}, nil
}

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

func missingSchedulerJobHTTPError(jobID string) *SystemHTTPError {
	details := map[string]any{"resource_type": "scheduler_job"}
	if jobID != "" {
		details["job_id"] = jobID
	}
	return MissingSystemResourceHTTPError(details)
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

func (h *SystemHandlers) HandleSystemSchedulerJobTrigger() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.scheduler == nil {
			WriteSystemHTTPError(w, r, missingSchedulerJobHTTPError(""))
			return
		}

		jobID := chi.URLParam(r, "job_id")
		response, systemErr := h.scheduler.TriggerJob(context.WithoutCancel(r.Context()), jobID)
		if systemErr != nil {
			WriteSystemHTTPError(w, r, systemErr)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, response)
	}
}
