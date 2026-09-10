package rayleabot

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"time"

	"github.com/RayleaBot/RayleaBot/sdk/go/internal/pluginwire"
)

const ProtocolVersion = pluginwire.ProtocolVersion

type Options struct {
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
	Logger        *slog.Logger
	ActionTimeout time.Duration
	ShutdownGrace time.Duration
}

type Handler interface {
	Handle(context.Context, *EventContext) error
}

type HandlerFunc func(context.Context, *EventContext) error

func (fn HandlerFunc) Handle(ctx context.Context, event *EventContext) error {
	return fn(ctx, event)
}

type Bot = pluginwire.BotIdentity

type Actor = pluginwire.ProtocolActorFrame

type Target = pluginwire.ProtocolTargetFrame

type Message = pluginwire.ProtocolMessageFrame

type Webhook = pluginwire.ProtocolWebhookFrame

// Event is the SDK view of a validated generated wire event. Payload remains
// an application-facing object; transport structs live in internal/pluginwire.
type Event struct {
	EventID        string
	SourceProtocol string
	SourceAdapter  string
	EventType      string
	Timestamp      int64
	Actor          Actor
	Target         Target
	Message        Message
	Webhook        *Webhook
	Payload        map[string]any
	Raw            json.RawMessage
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

type Segment = pluginwire.ProtocolSegmentFrame

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
