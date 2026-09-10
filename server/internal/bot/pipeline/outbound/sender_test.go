package outbound

import (
	"context"
	"errors"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
)

func TestUnconfirmedReplyNeverFallsBackToAnotherSend(t *testing.T) {
	failure := &chatevent.SendError{Code: errorcodes.AdapterSendUnconfirmed, Message: "receipt unavailable", Err: context.DeadlineExceeded}
	sender := &stubSender{replyErr: failure}
	_, err := SendAction(context.Background(), sender, stubReplyTargets{"event": {MessageID: "message", TargetType: "group", TargetID: "target", SourceProtocol: "qqofficial", SourceAdapter: "qq"}}, chatevent.Event{}, chatevent.MessageCommand{Kind: "message.reply", ReplyToEventID: "event", FallbackToSendIfMissing: true})
	if !errors.Is(err, failure) || sender.sendRequest.TargetID != "" {
		t.Fatalf("uncertain reply was retried: %#v %v", sender.sendRequest, err)
	}
}

type stubSender struct {
	sendRequest  chatevent.OutboundMessageSend
	replyRequest chatevent.OutboundMessageReply
	replyErr     error
}

func (s *stubSender) SendMessage(_ context.Context, request chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error) {
	s.sendRequest = request
	return chatevent.SendMessageResult{MessageID: "send-1"}, nil
}

func (s *stubSender) SendReply(_ context.Context, request chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error) {
	s.replyRequest = request
	if s.replyErr != nil {
		return chatevent.SendMessageResult{}, s.replyErr
	}
	return chatevent.SendMessageResult{MessageID: "reply-1"}, nil
}

type stubReplyTargets map[string]ReplyTarget

func (s stubReplyTargets) ResolveReplyTarget(eventID string) (ReplyTarget, bool) {
	target, ok := s[eventID]
	return target, ok
}

func TestSendActionRoutesMessageSend(t *testing.T) {
	t.Parallel()

	sender := &stubSender{}
	result, err := SendAction(context.Background(), sender, nil, chatevent.Event{}, chatevent.MessageCommand{
		Kind:       "message.send",
		TargetType: "group",
		TargetID:   "10001",
		MessageSegments: []chatevent.MessageSegment{
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
		replyErr: &chatevent.SendError{Code: codeAdapterReplyTargetMissing, Message: "missing"},
	}
	resolver := stubReplyTargets{
		"evt_1": {
			MessageID:  "msg_1",
			TargetType: "group",
			TargetID:   "10001",
		},
	}
	result, err := SendAction(context.Background(), sender, resolver, chatevent.Event{}, chatevent.MessageCommand{
		Kind:                    "message.reply",
		ReplyToEventID:          "evt_1",
		FallbackToSendIfMissing: true,
		MessageSegments: []chatevent.MessageSegment{
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
