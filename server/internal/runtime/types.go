package runtime

import (
	"context"
	"time"

	"rayleabot/server/internal/console"
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

type Snapshot struct {
	PluginID         string
	State            State
	LastErrorCode    string
	LastErrorMessage string
	InitRequestID    string
	PID              int
	StartedAt        *time.Time
	StoppedAt        *time.Time
	CrashCount       int
	NextRetryAt      *time.Time
	Subscriptions    []string
}

// CrashCallback is invoked by the runtime manager when a running plugin
// process exits unexpectedly. The lifecycle controller uses this to drive
// the backoff/restart cycle.
type CrashCallback func(pluginID string, crashCount int, lastErrorCode string)

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

type ActionSegment struct {
	Type string
	Data map[string]any
}

type Action struct {
	Kind                    string
	TargetType              string
	TargetID                string
	ReplyToEventID          string
	FallbackToSendIfMissing bool
	MessageSegments         []ActionSegment
	LogLevel                string
	LogMessage              string
	LogFields               map[string]any
	ConfigKeys              []string
	ConfigValues            map[string]any
	StorageOperation        string
	StorageRoot             string
	StoragePath             string
	StorageKey              string
	StoragePrefix           string
	StorageValue            any
	StorageContent          []byte
	HTTPMethod              string
	HTTPURL                 string
	HTTPHeaders             map[string]string
	HTTPTimeoutSeconds      int
	HTTPBody                []byte
	SchedulerTaskID         string
	SchedulerCron           string
	SchedulerEventType      string
	SchedulerPayload        map[string]any
	WebhookRoute            string
	WebhookMethods          []string
	WebhookAuthStrategy     string
	WebhookHeader           string
	WebhookSecretRef        string
	WebhookSignaturePrefix  string
	WebhookSourceIPs        []string
	RenderTemplate          string
	RenderTheme             string
	RenderOutput            string
	RenderFallbackText      string
	RenderData              map[string]any
}

type Delivery struct {
	RequestID    string
	Action       *Action
	Result       map[string]any
	ErrorCode    string
	ErrorMessage string
}

type managerDeps struct {
	now       func() time.Time
	requestID func() string
}

type LocalActionExecutor func(context.Context, string, string, Action) (map[string]any, error)

type Options struct {
	Console                    *console.Stream
	RedactText                 func(string) string
	StderrRateLimitBytesPerSec int
	OnCrash                    CrashCallback
	ExecuteLocalAction         LocalActionExecutor
}
