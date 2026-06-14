package outbound

import (
	"context"
	"testing"

	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

type stubSender struct {
	sendRequest  adapteroutbound.OutboundMessageSend
	replyRequest adapteroutbound.OutboundMessageReply
	replyErr     error
}

func (s *stubSender) SendMessage(_ context.Context, request adapteroutbound.OutboundMessageSend) (adapteroutbound.SendMessageResult, error) {
	s.sendRequest = request
	return adapteroutbound.SendMessageResult{MessageID: "send-1"}, nil
}

func (s *stubSender) SendReply(_ context.Context, request adapteroutbound.OutboundMessageReply) (adapteroutbound.SendMessageResult, error) {
	s.replyRequest = request
	if s.replyErr != nil {
		return adapteroutbound.SendMessageResult{}, s.replyErr
	}
	return adapteroutbound.SendMessageResult{MessageID: "reply-1"}, nil
}

type stubReplyTargets map[string]ReplyTarget

func (s stubReplyTargets) ResolveReplyTarget(eventID string) (ReplyTarget, bool) {
	target, ok := s[eventID]
	return target, ok
}

func TestSendActionRoutesMessageSend(t *testing.T) {
	t.Parallel()

	sender := &stubSender{}
	result, err := SendAction(context.Background(), sender, nil, runtimeprotocol.Event{}, runtimeaction.Action{
		Kind:       "message.send",
		TargetType: "group",
		TargetID:   "10001",
		MessageSegments: []runtimeaction.ActionSegment{
			{Type: "text", Data: map[string]any{"text": "hello"}},
		},
	})
	if err != nil {
		t.Fatalf("SendAction() error = %v", err)
	}
	if result.MessageID != "send-1" {
		t.Fatalf("message_id = %q, want send-1", result.MessageID)
	}
	if result.DeliveryKind != "message.send" {
		t.Fatalf("delivery_kind = %q, want message.send", result.DeliveryKind)
	}
	if sender.sendRequest.TargetID != "10001" {
		t.Fatalf("target_id = %q, want 10001", sender.sendRequest.TargetID)
	}
}

func TestSendActionFallsBackToSendWhenReplyTargetIsMissingAtAdapterLevel(t *testing.T) {
	t.Parallel()

	sender := &stubSender{
		replyErr: &adapteroutbound.Error{Code: codeAdapterReplyTargetMissing, Message: "missing"},
	}
	resolver := stubReplyTargets{
		"evt_1": {
			MessageID:  "msg_1",
			TargetType: "group",
			TargetID:   "10001",
		},
	}
	result, err := SendAction(context.Background(), sender, resolver, runtimeprotocol.Event{}, runtimeaction.Action{
		Kind:                    "message.reply",
		ReplyToEventID:          "evt_1",
		FallbackToSendIfMissing: true,
		MessageSegments: []runtimeaction.ActionSegment{
			{Type: "reply", Data: map[string]any{"id": "msg_1"}},
			{Type: "text", Data: map[string]any{"text": "fallback"}},
		},
	})
	if err != nil {
		t.Fatalf("SendAction() error = %v", err)
	}
	if result.MessageID != "send-1" {
		t.Fatalf("message_id = %q, want send-1", result.MessageID)
	}
	if result.DeliveryKind != "message.send" {
		t.Fatalf("delivery_kind = %q, want message.send", result.DeliveryKind)
	}
	if len(sender.sendRequest.Segments) != 1 || sender.sendRequest.Segments[0].Type != "text" {
		t.Fatalf("unexpected fallback segments: %#v", sender.sendRequest.Segments)
	}
}
