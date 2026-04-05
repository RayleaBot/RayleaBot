package outbound

import (
	"context"
	"testing"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/runtime"
)

type stubSender struct {
	sendRequest  adapter.OutboundMessageSend
	replyRequest adapter.OutboundMessageReply
	replyErr     error
}

func (s *stubSender) SendMessage(_ context.Context, request adapter.OutboundMessageSend) (adapter.SendMessageResult, error) {
	s.sendRequest = request
	return adapter.SendMessageResult{MessageID: "send-1"}, nil
}

func (s *stubSender) SendReply(_ context.Context, request adapter.OutboundMessageReply) (adapter.SendMessageResult, error) {
	s.replyRequest = request
	if s.replyErr != nil {
		return adapter.SendMessageResult{}, s.replyErr
	}
	return adapter.SendMessageResult{MessageID: "reply-1"}, nil
}

type stubReplyTargets map[string]ReplyTarget

func (s stubReplyTargets) ResolveReplyTarget(eventID string) (ReplyTarget, bool) {
	target, ok := s[eventID]
	return target, ok
}

func TestSendActionRoutesMessageSend(t *testing.T) {
	t.Parallel()

	sender := &stubSender{}
	result, err := SendAction(context.Background(), sender, nil, runtime.Event{}, runtime.Action{
		Kind:       "message.send",
		TargetType: "group",
		TargetID:   "10001",
		MessageSegments: []runtime.ActionSegment{
			{Type: "text", Data: map[string]any{"text": "hello"}},
		},
	})
	if err != nil {
		t.Fatalf("SendAction() error = %v", err)
	}
	if result.MessageID != "send-1" {
		t.Fatalf("message_id = %q, want send-1", result.MessageID)
	}
	if sender.sendRequest.TargetID != "10001" {
		t.Fatalf("target_id = %q, want 10001", sender.sendRequest.TargetID)
	}
}

func TestSendActionFallsBackToSendWhenReplyTargetIsMissingAtAdapterLevel(t *testing.T) {
	t.Parallel()

	sender := &stubSender{
		replyErr: &adapter.Error{Code: codeAdapterReplyTargetMissing, Message: "missing"},
	}
	resolver := stubReplyTargets{
		"evt_1": {
			MessageID:  "msg_1",
			TargetType: "group",
			TargetID:   "10001",
		},
	}
	result, err := SendAction(context.Background(), sender, resolver, runtime.Event{}, runtime.Action{
		Kind:                    "message.reply",
		ReplyToEventID:          "evt_1",
		FallbackToSendIfMissing: true,
		MessageSegments: []runtime.ActionSegment{
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
	if len(sender.sendRequest.Segments) != 1 || sender.sendRequest.Segments[0].Type != "text" {
		t.Fatalf("unexpected fallback segments: %#v", sender.sendRequest.Segments)
	}
}
