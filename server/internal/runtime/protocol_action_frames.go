package runtime

import "encoding/json"

type actionFrame struct {
	ProtocolVersion string          `json:"protocol_version"`
	Type            string          `json:"type"`
	Timestamp       int64           `json:"timestamp"`
	PluginID        string          `json:"plugin_id"`
	RequestID       string          `json:"request_id"`
	ParentRequestID string          `json:"parent_request_id,omitempty"`
	Action          string          `json:"action"`
	Data            json.RawMessage `json:"data"`
}

type protocolOutboundMessageFrame struct {
	Segments []protocolSegmentFrame `json:"segments"`
}

type protocolActionMessageSendFrame struct {
	TargetType string                        `json:"target_type"`
	TargetID   string                        `json:"target_id"`
	Message    *protocolOutboundMessageFrame `json:"message"`
}

type protocolActionMessageReplyFrame struct {
	ReplyToEventID          *string                       `json:"reply_to_event_id"`
	Message                 *protocolOutboundMessageFrame `json:"message"`
	FallbackToSendIfMissing bool                          `json:"fallback_to_send_if_missing,omitempty"`
}

type protocolActionLoggerWriteFrame struct {
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

type protocolActionStorageKVFrame struct {
	Operation string           `json:"operation"`
	Key       *string          `json:"key,omitempty"`
	Prefix    *string          `json:"prefix,omitempty"`
	Value     *json.RawMessage `json:"value,omitempty"`
}

type protocolActionConfigReadFrame struct {
	Keys []string `json:"keys"`
}

type protocolActionPluginListFrame struct {
	Visibility string `json:"visibility,omitempty"`
}

type protocolActionSecretReadFrame struct {
	Key string `json:"key"`
}

type protocolActionConfigWriteFrame struct {
	Values map[string]json.RawMessage `json:"values"`
}

type protocolActionGovernanceReadFrame struct{}

type protocolActionGovernanceBlacklistWriteFrame struct {
	Operation string  `json:"operation"`
	EntryType *string `json:"entry_type,omitempty"`
	TargetID  *string `json:"target_id,omitempty"`
	Reason    *string `json:"reason,omitempty"`
}

type protocolActionGovernanceWhitelistWriteFrame struct {
	Operation string  `json:"operation"`
	Enabled   *bool   `json:"enabled,omitempty"`
	EntryType *string `json:"entry_type,omitempty"`
	TargetID  *string `json:"target_id,omitempty"`
	Reason    *string `json:"reason,omitempty"`
}

type protocolActionStorageFileFrame struct {
	Operation     string  `json:"operation"`
	Root          string  `json:"root"`
	Path          *string `json:"path,omitempty"`
	Prefix        *string `json:"prefix,omitempty"`
	ContentText   *string `json:"content_text,omitempty"`
	ContentBase64 *string `json:"content_base64,omitempty"`
}

type protocolActionHTTPRequestFrame struct {
	Method         string            `json:"method"`
	URL            string            `json:"url"`
	Headers        map[string]string `json:"headers,omitempty"`
	TimeoutSeconds *int              `json:"timeout_seconds,omitempty"`
	BodyText       *string           `json:"body_text,omitempty"`
	BodyBase64     *string           `json:"body_base64,omitempty"`
}

type protocolActionSchedulerCreateFrame struct {
	TaskID    string          `json:"task_id"`
	LogLabel  string          `json:"log_label,omitempty"`
	Cron      string          `json:"cron"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type protocolActionEventExposeWebhookFrame struct {
	Route            string                                `json:"route"`
	Methods          []string                              `json:"methods"`
	AuthStrategy     string                                `json:"auth_strategy"`
	Header           string                                `json:"header"`
	SecretRef        string                                `json:"secret_ref"`
	SignaturePrefix  string                                `json:"signature_prefix,omitempty"`
	SourceIPs        []string                              `json:"source_ips,omitempty"`
	ReplayProtection *protocolWebhookReplayProtectionFrame `json:"replay_protection,omitempty"`
}

type protocolWebhookReplayProtectionFrame struct {
	TimestampHeader  string `json:"timestamp_header"`
	EventIDHeader    string `json:"event_id_header"`
	ToleranceSeconds int    `json:"tolerance_seconds"`
	Enforce          *bool  `json:"enforce"`
}

type protocolActionRenderImageFrame struct {
	Template     string          `json:"template"`
	Theme        string          `json:"theme,omitempty"`
	Output       string          `json:"output,omitempty"`
	FallbackText string          `json:"fallback_text,omitempty"`
	Data         json.RawMessage `json:"data"`
}
