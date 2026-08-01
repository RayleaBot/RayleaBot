package rayleabot

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"time"
)

const ProtocolVersion = "1"

type Options struct {
	PluginID              string
	Subscriptions         []string
	Stdin                 io.Reader
	Stdout                io.Writer
	Stderr                io.Writer
	Logger                *slog.Logger
	ActionTimeout         time.Duration
	ShutdownGrace         time.Duration
	MaxConcurrentHandlers int
}

type Handler interface {
	Handle(context.Context, *EventContext) error
}

type HandlerFunc func(context.Context, *EventContext) error

func (fn HandlerFunc) Handle(ctx context.Context, event *EventContext) error {
	return fn(ctx, event)
}

type Bot struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname,omitempty"`
}

type Permissions struct {
	SuperAdmins []string `json:"super_admins,omitempty"`
}

type Actor struct {
	ID       string `json:"id,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	Role     string `json:"role,omitempty"`
}

type Target struct {
	Type string `json:"type,omitempty"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Message struct {
	PlainText string          `json:"plain_text,omitempty"`
	Segments  []Segment       `json:"segments,omitempty"`
	Raw       json.RawMessage `json:"raw,omitempty"`
}

type Webhook struct {
	Route   string            `json:"route,omitempty"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

type Event struct {
	EventID        string          `json:"event_id,omitempty"`
	SourceProtocol string          `json:"source_protocol,omitempty"`
	SourceAdapter  string          `json:"source_adapter,omitempty"`
	EventType      string          `json:"event_type,omitempty"`
	Timestamp      int64           `json:"timestamp,omitempty"`
	Actor          Actor           `json:"actor,omitempty"`
	Target         Target          `json:"target,omitempty"`
	Message        Message         `json:"message,omitempty"`
	Webhook        *Webhook        `json:"webhook,omitempty"`
	Payload        map[string]any  `json:"payload,omitempty"`
	Raw            json.RawMessage `json:"-"`
}

func (event Event) Command() string {
	value, _ := event.Payload["command"].(string)
	return value
}

func (event Event) Args() []string {
	values, ok := event.Payload["args"].([]any)
	if !ok {
		if typed, ok := event.Payload["args"].([]string); ok {
			return append([]string(nil), typed...)
		}
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

type Segment struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

func Text(text string) Segment {
	return Segment{Type: "text", Data: map[string]any{"text": text}}
}

func Image(fileOrURL string) Segment {
	key := "file"
	if len(fileOrURL) >= 7 && (fileOrURL[:7] == "http://" || (len(fileOrURL) >= 8 && fileOrURL[:8] == "https://")) {
		key = "url"
	}
	return Segment{Type: "image", Data: map[string]any{key: fileOrURL}}
}

func At(userID string) Segment {
	return Segment{Type: "at", Data: map[string]any{"user_id": userID}}
}

func AtAll() Segment {
	return Segment{Type: "at_all"}
}

func Face(faceID string) Segment {
	return Segment{Type: "face", Data: map[string]any{"face_id": faceID}}
}

func Reply(messageID string) Segment {
	return Segment{Type: "reply", Data: map[string]any{"message_id": messageID}}
}

func Passthrough(segmentType string, data map[string]any) Segment {
	return Segment{Type: segmentType, Data: data}
}
