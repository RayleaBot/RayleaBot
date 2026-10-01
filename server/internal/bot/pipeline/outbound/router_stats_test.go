package outbound

import (
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"testing"
)

func TestRouterCountsOnlyConfirmedSendsIncludingReplyFallback(t *testing.T) {
	for _, tc := range []struct {
		name     string
		failure  error
		fallback bool
		want     int
	}{
		{name: "reply", want: 1},
		{name: "fallback", failure: &chatevent.SendError{Code: errorcodes.AdapterReplyTargetMissing}, fallback: true, want: 1},
		{name: "unconfirmed", failure: &chatevent.SendError{Code: errorcodes.AdapterSendUnconfirmed}, fallback: true},
		{name: "timeout", failure: context.DeadlineExceeded},
		{name: "rate limited", failure: &chatevent.SendError{Code: errorcodes.AdapterSendFailed}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			count := 0
			sender := &stubSender{replyErr: tc.failure}
			router := NewRouter(map[string]ActionSender{"bot": sender}, map[string]string{"bot": "onebot11"}, nil, func(id, protocol string) {
				if id != "bot" || protocol != "onebot11" {
					t.Fatalf("wrong attribution %s %s", id, protocol)
				}
				count++
			})
			_, _ = SendAction(t.Context(), router, stubReplyTargets{"event": {MessageID: "msg", TargetType: "group", TargetID: "group", SourceAdapter: "bot", SourceProtocol: "onebot11"}}, chatevent.Event{}, chatevent.MessageCommand{Kind: "message.reply", ReplyToEventID: "event", FallbackToSendIfMissing: tc.fallback})
			if count != tc.want {
				t.Fatalf("count=%d want=%d", count, tc.want)
			}
			_, _ = router.SendMessage(t.Context(), chatevent.OutboundMessageSend{SourceAdapter: "missing"})
			if count != tc.want {
				t.Fatal("unroutable send counted")
			}
			_, err := router.SendMessage(t.Context(), chatevent.OutboundMessageSend{SourceAdapter: "bot"})
			if err != nil || count != tc.want+1 {
				t.Fatalf("direct send not counted: %d %v", count, err)
			}
		})
	}
}
