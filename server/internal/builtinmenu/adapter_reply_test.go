package builtinmenu

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"testing"
)

type adapterReplySender struct {
	sent  chatevent.OutboundMessageSend
	reply chatevent.OutboundMessageReply
}

func (s *adapterReplySender) SendMessage(_ context.Context, m chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error) {
	s.sent = m
	return chatevent.SendMessageResult{}, nil
}
func (s *adapterReplySender) SendReply(_ context.Context, m chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error) {
	s.reply = m
	return chatevent.SendMessageResult{}, nil
}

func TestBuiltinResponseKeepsAdapter(t *testing.T) {
	sender := &adapterReplySender{}
	s := New(Deps{Sender: sender})
	s.sendBuiltinMenuText(context.Background(), chatevent.NormalizedEvent{SourceAdapter: "qq-official", SourceProtocol: "qqofficial", ConversationType: "group", ConversationID: "group-fixture", MessageID: "msg-fixture"}, "help", "fixture")
	if sender.reply.SourceAdapter != "qq-official" {
		t.Fatalf("menu reply source_adapter=%q, want qq-official for routing", sender.reply.SourceAdapter)
	}
}

func TestBuiltinC2CUsesPassiveReply(t *testing.T) {
	sender := &adapterReplySender{}
	s := New(Deps{Sender: sender})
	s.sendBuiltinMenuText(context.Background(), chatevent.NormalizedEvent{SourceAdapter: "qq-official", SourceProtocol: "qqofficial", ConversationType: "private", ConversationID: "user-fixture", MessageID: "msg-fixture"}, "help", "fixture")
	if sender.reply.ReplyToMessageID != "msg-fixture" {
		t.Fatalf("C2C menu used active send target=%s instead of quoting incoming msg_id", sender.sent.TargetID)
	}
}
