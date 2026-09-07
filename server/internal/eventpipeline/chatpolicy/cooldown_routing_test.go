package chatpolicy

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"testing"
)

type cooldownRoutingRecorder struct {
	send  chatevent.OutboundMessageSend
	reply chatevent.OutboundMessageReply
}

func (s *cooldownRoutingRecorder) SendMessage(_ context.Context, message chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error) {
	s.send = message
	return chatevent.SendMessageResult{}, nil
}
func (s *cooldownRoutingRecorder) SendReply(_ context.Context, message chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error) {
	s.reply = message
	return chatevent.SendMessageResult{}, nil
}

func TestCooldownReplyKeepsInstanceAndPassiveQQMessage(t *testing.T) {
	for _, testCase := range []struct {
		protocol, target string
		reply            bool
	}{
		{"onebot11", "group", true}, {"onebot11", "private", false}, {"qqofficial", "group", true}, {"qqofficial", "private", true},
	} {
		t.Run(testCase.protocol+"/"+testCase.target, func(t *testing.T) {
			sender := &cooldownRoutingRecorder{}
			service := New(Deps{OutboundSender: sender})
			service.sendCooldownReply(context.Background(), chatevent.NormalizedEvent{SourceAdapter: "second-bot", SourceProtocol: testCase.protocol, ConversationType: testCase.target, ConversationID: "301", MessageID: "42"})
			if testCase.reply {
				if sender.reply.SourceAdapter != "second-bot" || sender.reply.SourceProtocol != testCase.protocol || sender.reply.ReplyToMessageID != "42" {
					t.Fatalf("reply=%+v", sender.reply)
				}
			} else if sender.send.SourceAdapter != "second-bot" || sender.send.SourceProtocol != testCase.protocol {
				t.Fatalf("send=%+v", sender.send)
			}
		})
	}
}
