package actions_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

type messageSendRecorder struct {
	messages []onebot11.OutboundMessageSend
}

func (r *messageSendRecorder) SendMessage(_ context.Context, message onebot11.OutboundMessageSend) (onebot11.SendMessageResult, error) {
	r.messages = append(r.messages, message)
	return onebot11.SendMessageResult{MessageID: "message-1"}, nil
}

func (r *messageSendRecorder) SendReply(context.Context, onebot11.OutboundMessageReply) (onebot11.SendMessageResult, error) {
	return onebot11.SendMessageResult{}, nil
}

func TestMessageSendLocalActionUsesSharedOutboundPath(t *testing.T) {
	t.Parallel()

	recorder := &messageSendRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dispatcher := dispatch.New(logger, recorder, nil, 16)
	dispatcher.SetCapabilityChecker(func(context.Context, string, string) bool { return true })
	defer dispatcher.Close()

	service := actions.New(actions.Deps{
		Capabilities:  &stubCapabilityView{capabilities: map[string]bool{"message.send": true}},
		MessageSender: actions.OutboundMessageSender(dispatcher),
	})
	action := pluginruntime.Action{
		Kind:       "message.send",
		TargetType: "group",
		TargetID:   "2001",
		MessageSegments: []pluginruntime.ActionSegment{{
			Type: "text",
			Data: map[string]any{"text": "正在处理"},
		}},
	}

	result, err := service.Execute(context.Background(), "guide-plugin", "local-message-1", action, pluginruntime.Event{
		EventID:   "event-1",
		EventType: "message.group",
	})
	if err != nil {
		t.Fatalf("execute message.send: %v", err)
	}
	if result["message_id"] != "message-1" || result["delivery_kind"] != "message.send" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(recorder.messages) != 1 {
		t.Fatalf("outbound sends = %d, want 1", len(recorder.messages))
	}
	message := recorder.messages[0]
	if message.TargetType != "group" || message.TargetID != "2001" || len(message.Segments) != 1 {
		t.Fatalf("unexpected outbound message: %#v", message)
	}
}
