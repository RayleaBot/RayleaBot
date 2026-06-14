package systemapi

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
