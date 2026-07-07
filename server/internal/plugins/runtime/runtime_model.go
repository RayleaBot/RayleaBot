package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/console"
)

type State string

const (
	StateStopped    State = "stopped"
	StateStarting   State = "starting"
	StateRunning    State = "running"
	StateStopping   State = "stopping"
	StateCrashed    State = "crashed"
	StateBackoff    State = "backoff"
	StateDeadLetter State = "dead_letter"
)

const (
	codePlatformInvalidRequest  = "platform.invalid_request"
	codePlatformResourceMissing = "platform.resource_missing"
	codePluginInitTimeout       = "plugin.init_timeout"
	codePluginEventTimeout      = "plugin.event_timeout"
	codePluginInternalError     = "plugin.internal_error"
	codePluginNotHandled        = "plugin.not_handled"
	codePluginProtocolViolation = "plugin.protocol_violation"
	codePluginShutdownTimeout   = "plugin.shutdown_timeout"
	codePluginStopping          = "plugin.stopping"
)

type Error struct {
	Code    string
	Message string
	Details map[string]any
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func errorf(code, message string, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

func errorWithDetails(code, message string, details map[string]any, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Details: cloneDetails(details),
		Err:     err,
	}
}

func cloneDetails(details map[string]any) map[string]any {
	if len(details) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(details))
	for key, value := range details {
		cloned[key] = value
	}
	return cloned
}

type Snapshot struct {
	PluginID            string
	State               State
	LastErrorCode       string
	LastErrorMessage    string
	InitRequestID       string
	PID                 int
	StartedAt           *time.Time
	StoppedAt           *time.Time
	CrashCount          int
	NextRetryAt         *time.Time
	EnteredDeadLetterAt *time.Time
	Subscriptions       []string
}

type Delivery struct {
	RequestID    string
	Action       *Action
	Result       map[string]any
	ErrorCode    string
	ErrorMessage string
	ErrorDetails map[string]any
}

type Event struct {
	EventID        string
	SourceProtocol string
	SourceAdapter  string
	EventType      string
	Timestamp      int64
	Actor          *EventActor
	Target         *EventTarget
	Message        *EventMessage
	PayloadFields  map[string]any
	MessageID      string
	RawPayload     any
	SchedulerLog   *SchedulerLogContext
}

type SchedulerLogContext struct {
	JobID      string
	PluginName string
	TaskName   string
	LogLabel   string
	StartedAt  time.Time
	Recorder   SchedulerRunRecorder
}

type SchedulerRunRecorder interface {
	RecordSchedulerRunResult(context.Context, SchedulerRunResult) error
}

type SchedulerRunResult struct {
	JobID      string
	Outcome    string
	Duration   time.Duration
	ErrorCode  string
	ErrorText  string
	OccurredAt time.Time
}

type EventActor struct {
	ID       string
	Nickname string
	Role     string
}

type EventTarget struct {
	Type string
	ID   string
	Name string
}

type EventMessage struct {
	PlainText string
	Segments  []EventSegment
}

type EventSegment struct {
	Type string
	Data map[string]any
}

type EventFrame struct {
	ProtocolVersion string             `json:"protocol_version"`
	Type            string             `json:"type"`
	Timestamp       int64              `json:"timestamp"`
	PluginID        string             `json:"plugin_id"`
	RequestID       string             `json:"request_id"`
	Event           ProtocolEventFrame `json:"event"`
}

type ProtocolEventFrame struct {
	EventID        string                `json:"event_id"`
	SourceProtocol string                `json:"source_protocol"`
	SourceAdapter  string                `json:"source_adapter"`
	EventType      string                `json:"event_type"`
	Timestamp      int64                 `json:"timestamp"`
	Actor          *ProtocolActorFrame   `json:"actor,omitempty"`
	Target         *ProtocolTargetFrame  `json:"target,omitempty"`
	Message        *ProtocolMessageFrame `json:"message,omitempty"`
	Payload        *ProtocolPayloadFrame `json:"payload,omitempty"`
	RawPayload     any                   `json:"raw_payload,omitempty"`
}

type ProtocolActorFrame struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname,omitempty"`
	Role     string `json:"role,omitempty"`
}

type ProtocolTargetFrame struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type ProtocolMessageFrame struct {
	PlainText string                 `json:"plain_text,omitempty"`
	Segments  []ProtocolSegmentFrame `json:"segments,omitempty"`
}

type ProtocolSegmentFrame struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

type ProtocolPayloadFrame struct {
	MessageID  string                        `json:"message_id,omitempty"`
	Command    string                        `json:"command,omitempty"`
	Args       []string                      `json:"args,omitempty"`
	SubType    string                        `json:"sub_type,omitempty"`
	OperatorID string                        `json:"operator_id,omitempty"`
	OneBot     *ProtocolOneBotPayloadFrame   `json:"onebot,omitempty"`
	Bilibili   *ProtocolBilibiliPayloadFrame `json:"bilibili,omitempty"`
}

type ProtocolBilibiliPayloadFrame struct {
	Kind           string                         `json:"kind"`
	UID            string                         `json:"uid"`
	ID             string                         `json:"id"`
	RoomID         string                         `json:"room_id,omitempty"`
	Service        string                         `json:"service"`
	Title          string                         `json:"title,omitempty"`
	Summary        string                         `json:"summary,omitempty"`
	SummaryHTML    string                         `json:"summary_html,omitempty"`
	URL            string                         `json:"url"`
	PubTS          int64                          `json:"pub_ts,omitempty"`
	CreatedAt      string                         `json:"created_at,omitempty"`
	Author         ProtocolBilibiliAuthorFrame    `json:"author"`
	Images         []ProtocolBilibiliImageFrame   `json:"images,omitempty"`
	Topic          *ProtocolBilibiliTopicFrame    `json:"topic,omitempty"`
	Original       *ProtocolBilibiliOriginalFrame `json:"original,omitempty"`
	LiveStatus     *int                           `json:"live_status,omitempty"`
	LiveEvent      string                         `json:"live_event,omitempty"`
	StatusLabel    string                         `json:"status_label,omitempty"`
	LiveStartedAt  string                         `json:"live_started_at,omitempty"`
	LiveDetectedAt string                         `json:"live_detected_at,omitempty"`
	DynamicType    string                         `json:"dynamic_type,omitempty"`
}

type ProtocolBilibiliOriginalFrame struct {
	ID          string                       `json:"id"`
	Service     string                       `json:"service"`
	Title       string                       `json:"title,omitempty"`
	Summary     string                       `json:"summary,omitempty"`
	SummaryHTML string                       `json:"summary_html,omitempty"`
	URL         string                       `json:"url"`
	PubTS       int64                        `json:"pub_ts,omitempty"`
	CreatedAt   string                       `json:"created_at,omitempty"`
	Author      ProtocolBilibiliAuthorFrame  `json:"author"`
	Images      []ProtocolBilibiliImageFrame `json:"images,omitempty"`
	Topic       *ProtocolBilibiliTopicFrame  `json:"topic,omitempty"`
	DynamicType string                       `json:"dynamic_type,omitempty"`
}

type ProtocolBilibiliTopicFrame struct {
	ID      int64  `json:"id,omitempty"`
	Name    string `json:"name"`
	JumpURL string `json:"jump_url,omitempty"`
}

type ProtocolBilibiliAuthorFrame struct {
	UID    string `json:"uid"`
	Name   string `json:"name"`
	Avatar string `json:"avatar,omitempty"`
}

type ProtocolBilibiliImageFrame struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

type ProtocolOneBotPayloadFrame struct {
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
	Sender        *ProtocolOneBotSenderFrame `json:"sender,omitempty"`
	Comment       string                     `json:"comment,omitempty"`
	Flag          string                     `json:"flag,omitempty"`
	Status        map[string]any             `json:"status,omitempty"`
}

type ProtocolOneBotSenderFrame struct {
	UserID   string `json:"user_id,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Card     string `json:"card,omitempty"`
	Role     string `json:"role,omitempty"`
	Title    string `json:"title,omitempty"`
	Sex      string `json:"sex,omitempty"`
	Age      int    `json:"age,omitempty"`
}

type PingFrame struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
}

type InitFrame struct {
	ProtocolVersion string            `json:"protocol_version"`
	Type            string            `json:"type"`
	Timestamp       int64             `json:"timestamp"`
	PluginID        string            `json:"plugin_id"`
	RequestID       string            `json:"request_id"`
	Bot             *BotFrame         `json:"bot,omitempty"`
	Capabilities    []string          `json:"capabilities,omitempty"`
	Permissions     *PermissionsFrame `json:"permissions,omitempty"`
	CommandPrefixes []string          `json:"command_prefixes"`
}

type BotFrame struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname,omitempty"`
}

type PermissionsFrame struct {
	SuperAdmins []string `json:"super_admins,omitempty"`
}

type ShutdownFrame struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
	Reason          string `json:"reason"`
}

type FrameEnvelope struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
}

type InitProgressFrame struct {
	ProtocolVersion string `json:"protocol_version"`
	Type            string `json:"type"`
	Timestamp       int64  `json:"timestamp"`
	PluginID        string `json:"plugin_id"`
	RequestID       string `json:"request_id"`
	Summary         string `json:"summary"`
}

type InitAckFrame struct {
	Type          string   `json:"type"`
	RequestID     string   `json:"request_id"`
	Status        string   `json:"status"`
	Subscriptions []string `json:"subscriptions,omitempty"`
	ErrorMessage  string   `json:"error_message,omitempty"`
}

type ErrorFrame struct {
	Type      string         `json:"type"`
	RequestID string         `json:"request_id"`
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
}

type ResultFrame struct {
	Type      string         `json:"type"`
	RequestID string         `json:"request_id"`
	Status    string         `json:"status"`
	Data      map[string]any `json:"data"`
}

type InitResponseStatus int

const (
	InitResponseWait InitResponseStatus = iota
	InitResponseReady
)

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

type ProtocolActionThirdPartyAccountReadFrame struct {
	Platform  string `json:"platform"`
	AccountID string `json:"account_id,omitempty"`
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

// CrashCallback is invoked by the runtime manager when a running plugin
// process exits unexpectedly. The lifecycle controller uses this to drive
// the backoff/restart cycle.
type CrashCallback func(pluginID string, crashCount int, lastErrorCode string)

type managerDeps struct {
	now       func() time.Time
	requestID func() string
}

type LocalActionExecutor func(context.Context, string, string, Action, Event) (map[string]any, error)

type Options struct {
	Console                    *console.Stream
	RedactText                 func(string) string
	StderrRateLimitBytesPerSec int
	OnCrash                    CrashCallback
	ExecuteLocalAction         LocalActionExecutor
}
