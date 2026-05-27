package runtime

import "encoding/json"

type pingFrame struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
}

type initFrame struct {
	ProtocolVersion string            `json:"protocol_version"`
	Type            string            `json:"type"`
	Timestamp       int64             `json:"timestamp"`
	PluginID        string            `json:"plugin_id"`
	RequestID       string            `json:"request_id"`
	Bot             *botFrame         `json:"bot,omitempty"`
	Capabilities    []string          `json:"capabilities,omitempty"`
	Permissions     *permissionsFrame `json:"permissions,omitempty"`
	CommandPrefixes []string          `json:"command_prefixes"`
}

type botFrame struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname,omitempty"`
}

type permissionsFrame struct {
	SuperAdmins []string `json:"super_admins,omitempty"`
}

type shutdownFrame struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
	Reason          string `json:"reason"`
}

type eventFrame struct {
	ProtocolVersion string             `json:"protocol_version"`
	Type            string             `json:"type"`
	Timestamp       int64              `json:"timestamp"`
	PluginID        string             `json:"plugin_id"`
	RequestID       string             `json:"request_id"`
	Event           protocolEventFrame `json:"event"`
}

type protocolEventFrame struct {
	EventID        string                `json:"event_id"`
	SourceProtocol string                `json:"source_protocol"`
	SourceAdapter  string                `json:"source_adapter"`
	EventType      string                `json:"event_type"`
	Timestamp      int64                 `json:"timestamp"`
	Actor          *protocolActorFrame   `json:"actor,omitempty"`
	Target         *protocolTargetFrame  `json:"target,omitempty"`
	Message        *protocolMessageFrame `json:"message,omitempty"`
	Payload        *protocolPayloadFrame `json:"payload,omitempty"`
	RawPayload     any                   `json:"raw_payload,omitempty"`
}

type protocolActorFrame struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname,omitempty"`
	Role     string `json:"role,omitempty"`
}

type protocolTargetFrame struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type protocolMessageFrame struct {
	PlainText string                 `json:"plain_text,omitempty"`
	Segments  []protocolSegmentFrame `json:"segments,omitempty"`
}

type protocolSegmentFrame struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

type protocolPayloadFrame struct {
	MessageID  string                      `json:"message_id,omitempty"`
	Command    string                      `json:"command,omitempty"`
	Args       []string                    `json:"args,omitempty"`
	SubType    string                      `json:"sub_type,omitempty"`
	OperatorID string                      `json:"operator_id,omitempty"`
	OneBot     *protocolOneBotPayloadFrame `json:"onebot,omitempty"`
}

type protocolOneBotPayloadFrame struct {
	PostType      string                     `json:"post_type,omitempty"`
	MetaEventType string                     `json:"meta_event_type,omitempty"`
	MessageType   string                     `json:"message_type,omitempty"`
	RequestType   string                     `json:"request_type,omitempty"`
	NoticeType    string                     `json:"notice_type,omitempty"`
	SubType       string                     `json:"sub_type,omitempty"`
	SelfID        string                     `json:"self_id,omitempty"`
	UserID        string                     `json:"user_id,omitempty"`
	GroupID       string                     `json:"group_id,omitempty"`
	TargetID      string                     `json:"target_id,omitempty"`
	Time          int64                      `json:"time,omitempty"`
	Interval      int                        `json:"interval,omitempty"`
	MessageID     string                     `json:"message_id,omitempty"`
	RealID        string                     `json:"real_id,omitempty"`
	MessageSeq    string                     `json:"message_seq,omitempty"`
	RawMessage    string                     `json:"raw_message,omitempty"`
	Font          int                        `json:"font,omitempty"`
	MessageFormat string                     `json:"message_format,omitempty"`
	Sender        *protocolOneBotSenderFrame `json:"sender,omitempty"`
	Comment       string                     `json:"comment,omitempty"`
	Flag          string                     `json:"flag,omitempty"`
	Status        map[string]any             `json:"status,omitempty"`
}

type protocolOneBotSenderFrame struct {
	UserID   string `json:"user_id,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Card     string `json:"card,omitempty"`
	Role     string `json:"role,omitempty"`
	Title    string `json:"title,omitempty"`
	Sex      string `json:"sex,omitempty"`
	Age      int    `json:"age,omitempty"`
}

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

type frameEnvelope struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
}

type initProgressFrame struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
	Summary         string `json:"summary"`
}

type initAckFrame struct {
	Type          string   `json:"type"`
	RequestID     string   `json:"request_id"`
	Status        string   `json:"status"`
	Subscriptions []string `json:"subscriptions,omitempty"`
	ErrorMessage  string   `json:"error_message,omitempty"`
}

type errorFrame struct {
	Type      string         `json:"type"`
	RequestID string         `json:"request_id"`
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
}

type resultFrame struct {
	Type      string         `json:"type"`
	RequestID string         `json:"request_id"`
	Status    string         `json:"status"`
	Data      map[string]any `json:"data"`
}

type initResponseStatus int

const (
	initResponseWait initResponseStatus = iota
	initResponseReady
)
