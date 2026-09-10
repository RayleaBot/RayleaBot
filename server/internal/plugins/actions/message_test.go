package actions_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
)

type messageSendRecorder struct {
	messages []chatevent.OutboundMessageSend
}

func (r *messageSendRecorder) SendMessage(_ context.Context, message chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error) {
	r.messages = append(r.messages, message)
	return chatevent.SendMessageResult{MessageID: "message-1"}, nil
}

func (r *messageSendRecorder) SendReply(context.Context, chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error) {
	return chatevent.SendMessageResult{}, nil
}

func TestMessageSendLocalActionUsesSharedOutboundPath(t *testing.T) {
	t.Parallel()

	recorder := &messageSendRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dispatcher := dispatch.New(logger, recorder, nil, 16)
	dispatcher.SetPermissionChecker(func(context.Context, string, string) bool { return true })
	defer dispatcher.Close()

	service := actions.New(actions.Deps{
		Permissions:   &stubPermissionView{permissions: map[string]bool{"message.send": true}},
		MessageSender: actions.OutboundMessageSender(dispatcher),
	})
	action := plugins.Action{
		Kind:       "message.send",
		TargetType: "group",
		TargetID:   "2001",
		MessageSegments: []chatevent.MessageSegment{{
			Type: "text",
			Data: map[string]any{"text": "正在处理"},
		}},
	}

	result, err := service.Execute(context.Background(), "guide-plugin", "local-message-1", action, chatevent.Event{
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
