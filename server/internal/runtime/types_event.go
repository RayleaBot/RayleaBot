package runtime

import (
	"context"
	"time"
)

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
