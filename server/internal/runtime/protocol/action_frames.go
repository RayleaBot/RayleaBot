package runtimeprotocol

import "encoding/json"

type ActionFrame struct {
	ProtocolVersion string          `json:"protocol_version"`
	Type            string          `json:"type"`
	Timestamp       int64           `json:"timestamp"`
	PluginID        string          `json:"plugin_id"`
	RequestID       string          `json:"request_id"`
	ParentRequestID string          `json:"parent_request_id,omitempty"`
	Action          string          `json:"action"`
	Data            json.RawMessage `json:"data"`
}

type ProtocolOutboundMessageFrame struct {
	Segments []ProtocolSegmentFrame `json:"segments"`
}

type ProtocolActionMessageSendFrame struct {
	TargetType string                        `json:"target_type"`
	TargetID   string                        `json:"target_id"`
	Message    *ProtocolOutboundMessageFrame `json:"message"`
}

type ProtocolActionMessageReplyFrame struct {
	ReplyToEventID          *string                       `json:"reply_to_event_id"`
	Message                 *ProtocolOutboundMessageFrame `json:"message"`
	FallbackToSendIfMissing bool                          `json:"fallback_to_send_if_missing,omitempty"`
}

type ProtocolActionLoggerWriteFrame struct {
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

type ProtocolActionStorageKVFrame struct {
	Operation string           `json:"operation"`
	Key       *string          `json:"key,omitempty"`
	Prefix    *string          `json:"prefix,omitempty"`
	Value     *json.RawMessage `json:"value,omitempty"`
}

type ProtocolActionConfigReadFrame struct {
	Keys []string `json:"keys"`
}

type ProtocolActionPluginListFrame struct {
	Visibility string `json:"visibility,omitempty"`
}

type ProtocolActionSecretReadFrame struct {
	Key string `json:"key"`
}

type ProtocolActionConfigWriteFrame struct {
	Values map[string]json.RawMessage `json:"values"`
}

type ProtocolActionGovernanceReadFrame struct{}

type ProtocolActionGovernanceBlacklistWriteFrame struct {
	Operation string  `json:"operation"`
	EntryType *string `json:"entry_type,omitempty"`
	TargetID  *string `json:"target_id,omitempty"`
	Reason    *string `json:"reason,omitempty"`
}

type ProtocolActionGovernanceWhitelistWriteFrame struct {
	Operation string  `json:"operation"`
	Enabled   *bool   `json:"enabled,omitempty"`
	EntryType *string `json:"entry_type,omitempty"`
	TargetID  *string `json:"target_id,omitempty"`
	Reason    *string `json:"reason,omitempty"`
}

type ProtocolActionStorageFileFrame struct {
	Operation     string  `json:"operation"`
	Root          string  `json:"root"`
	Path          *string `json:"path,omitempty"`
	Prefix        *string `json:"prefix,omitempty"`
	ContentText   *string `json:"content_text,omitempty"`
	ContentBase64 *string `json:"content_base64,omitempty"`
}

type ProtocolActionHTTPRequestFrame struct {
	Method         string            `json:"method"`
	URL            string            `json:"url"`
	Headers        map[string]string `json:"headers,omitempty"`
	TimeoutSeconds *int              `json:"timeout_seconds,omitempty"`
	BodyText       *string           `json:"body_text,omitempty"`
	BodyBase64     *string           `json:"body_base64,omitempty"`
}

type ProtocolActionSchedulerCreateFrame struct {
	TaskID    string          `json:"task_id"`
	LogLabel  string          `json:"log_label,omitempty"`
	Cron      string          `json:"cron"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type ProtocolActionEventExposeWebhookFrame struct {
	Route            string                                `json:"route"`
	Methods          []string                              `json:"methods"`
	AuthStrategy     string                                `json:"auth_strategy"`
	Header           string                                `json:"header"`
	SecretRef        string                                `json:"secret_ref"`
	SignaturePrefix  string                                `json:"signature_prefix,omitempty"`
	SourceIPs        []string                              `json:"source_ips,omitempty"`
	ReplayProtection *ProtocolWebhookReplayProtectionFrame `json:"replay_protection,omitempty"`
}

type ProtocolWebhookReplayProtectionFrame struct {
	TimestampHeader  string `json:"timestamp_header"`
	EventIDHeader    string `json:"event_id_header"`
	ToleranceSeconds int    `json:"tolerance_seconds"`
	Enforce          *bool  `json:"enforce"`
}

type ProtocolActionRenderImageFrame struct {
	Template     string          `json:"template"`
	Theme        string          `json:"theme,omitempty"`
	Output       string          `json:"output,omitempty"`
	FallbackText string          `json:"fallback_text,omitempty"`
	Data         json.RawMessage `json:"data"`
}
