package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/RayleaBot/RayleaBot/server/internal/pagination"
	"sort"
	"strings"
	"time"
)

type JobList struct {
	pagination.Metadata
	Items []JobSummary `json:"items"`
}

type JobSummary struct {
	JobID          string            `json:"job_id"`
	PluginID       string            `json:"plugin_id"`
	PluginName     string            `json:"plugin_name"`
	TaskName       string            `json:"task_name"`
	LogLabel       string            `json:"log_label"`
	CronExpr       string            `json:"cron_expr"`
	Timezone       string            `json:"timezone"`
	Enabled        bool              `json:"enabled"`
	NextRun        string            `json:"next_run"`
	LastRun        *string           `json:"last_run"`
	LastDurationMS int64             `json:"last_duration_ms"`
	LastError      *JobLastError     `json:"last_error,omitempty"`
	PayloadSummary JobPayloadSummary `json:"payload_summary"`
	Stats          JobRunStats       `json:"stats"`
}

type JobLastError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	At      string `json:"at"`
}

type JobPayloadSummary struct {
	ConversationID string `json:"conversation_id"`
	TargetType     string `json:"target_type"`
	TargetID       string `json:"target_id"`
	Content        string `json:"content"`
}

type JobRunStats struct {
	Total   int64 `json:"total"`
	Success int64 `json:"success"`
	Failed  int64 `json:"failed"`
	Timeout int64 `json:"timeout"`
	Retry   int64 `json:"retry"`
	Other   int64 `json:"other"`
}

type TriggerResult struct {
	JobID     string `json:"job_id"`
	PluginID  string `json:"plugin_id"`
	Triggered bool   `json:"triggered"`
}

// View builds scheduler-owned summaries from the engine's active timezone.
// It shares the engine's lifecycle and holds no background resources.
type View struct {
	engine     *Engine
	pluginName func(string) string
}

func NewView(engine *Engine, pluginName func(string) string) (*View, error) {
	if engine == nil {
		return nil, errors.New("scheduler engine is required")
	}
	return &View{engine: engine, pluginName: pluginName}, nil
}

func (s *View) ListJobs() JobList {
	jobs := s.engine.Jobs()
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].PluginID == jobs[j].PluginID {
			return jobs[i].JobID < jobs[j].JobID
		}
		return jobs[i].PluginID < jobs[j].PluginID
	})
	items := make([]JobSummary, 0, len(jobs))
	for _, job := range jobs {
		items = append(items, s.jobSummary(job))
	}
	return JobList{Metadata: pagination.Metadata{Total: len(items)}, Items: items}
}

func (s *View) TriggerJob(ctx context.Context, jobID string) (TriggerResult, error) {
	job, err := s.engine.Trigger(ctx, jobID)
	if err != nil {
		return TriggerResult{}, err
	}
	return TriggerResult{JobID: job.JobID, PluginID: job.PluginID, Triggered: true}, nil
}

func (s *View) jobSummary(job Job) JobSummary {
	pluginName := strings.TrimSpace(job.PluginID)
	if s.pluginName != nil {
		pluginName = DisplayLabel(s.pluginName(job.PluginID), pluginName)
	}
	if pluginName == "" {
		pluginName = "未知插件"
	}
	lastRun := formatOptionalTime(job.LastRun)
	var lastError *JobLastError
	if job.LastError != nil && (job.LastError.Code != "" || job.LastError.Message != "") {
		at := job.LastError.At
		if at.IsZero() {
			at = time.Now().UTC()
		}
		lastError = &JobLastError{
			Code:    job.LastError.Code,
			Message: job.LastError.Message,
			At:      at.UTC().Format(time.RFC3339),
		}
	}
	return JobSummary{
		JobID:          job.JobID,
		PluginID:       job.PluginID,
		PluginName:     pluginName,
		TaskName:       DisplayLabel(job.JobID, "未命名任务"),
		LogLabel:       DisplayLabel(job.LogLabel),
		CronExpr:       job.CronExpr,
		Timezone:       s.engine.Timezone(),
		Enabled:        job.Enabled,
		NextRun:        job.NextRun.UTC().Format(time.RFC3339),
		LastRun:        lastRun,
		LastDurationMS: job.LastDurationMS,
		LastError:      lastError,
		PayloadSummary: summarizeSchedulerPayload(job.Payload),
		Stats: JobRunStats{
			Total:   job.RunStats.Total(),
			Success: job.RunStats.Success,
			Failed:  job.RunStats.Failed,
			Timeout: job.RunStats.Timeout,
			Retry:   job.RunStats.Retry,
			Other:   job.RunStats.Other,
		},
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func summarizeSchedulerPayload(raw json.RawMessage) JobPayloadSummary {
	var payload map[string]any
	if len(raw) == 0 || string(raw) == "null" {
		return JobPayloadSummary{}
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return JobPayloadSummary{}
	}
	targetType := firstPayloadText(payload, "target_type", "type")
	targetID := firstPayloadText(payload, "target_id", "group_id", "user_id", "conversation_id")
	conversationID := firstPayloadText(payload, "conversation_id", "session_id")
	if conversationID == "" && targetType != "" && targetID != "" {
		conversationID = targetType + ":" + targetID
	}
	return JobPayloadSummary{
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
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

type JobQuery struct {
	pagination.Query
	Status string
	Sort   string
}

func (s *View) ListJobsPage(query JobQuery) JobList {
	list := s.ListJobs()
	filtered := make([]JobSummary, 0, len(list.Items))
	for _, item := range list.Items {
		if query.Status == "success" && item.LastError != nil || query.Status == "error" && item.LastError == nil {
			continue
		}
		if pagination.Matches(query.Text, item.JobID, item.PluginID, item.PluginName, item.TaskName, item.LogLabel, item.PayloadSummary.Content) {
			filtered = append(filtered, item)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		left, right := filtered[i], filtered[j]
		switch query.Sort {
		case "last_run":
			if left.LastRun == nil && right.LastRun != nil {
				return false
			}
			if right.LastRun == nil && left.LastRun != nil {
				return true
			}
			if left.LastRun != nil && right.LastRun != nil && *left.LastRun != *right.LastRun {
				return *left.LastRun > *right.LastRun
			}
		case "duration":
			if left.LastDurationMS != right.LastDurationMS {
				return left.LastDurationMS > right.LastDurationMS
			}
		default:
			if a, b := strings.ToLower(left.PluginName), strings.ToLower(right.PluginName); a != b {
				return a < b
			}
			if a, b := strings.ToLower(left.TaskName), strings.ToLower(right.TaskName); a != b {
				return a < b
			}
		}
		if left.PluginID != right.PluginID {
			return left.PluginID < right.PluginID
		}
		return left.JobID < right.JobID
	})
	items, meta := pagination.Slice(filtered, query.Query)
	return JobList{Metadata: meta, Items: items}
}
