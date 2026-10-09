package outbound

import (
	"context"
	"errors"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

type recordingSender struct {
	name string
	sent *[]string
}

func (s recordingSender) SendMessage(context.Context, chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error) {
	*s.sent = append(*s.sent, s.name)
	return chatevent.SendMessageResult{MessageID: s.name}, nil
}

func (s recordingSender) SendReply(context.Context, chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error) {
	*s.sent = append(*s.sent, s.name+"-reply")
	return chatevent.SendMessageResult{MessageID: s.name}, nil
}

func TestAdapterRouterDeliversThroughTheOriginatingProtocol(t *testing.T) {
	t.Parallel()

	var sent []string
	router := NewRouter(map[string]ActionSender{
		"onebot11":    recordingSender{name: "onebot11", sent: &sent},
		"qq-official": recordingSender{name: "qqofficial", sent: &sent},
	}, map[string]string{"onebot11": "onebot11", "qq-official": "qqofficial"}, nil)

	if _, err := router.SendReply(context.Background(), chatevent.OutboundMessageReply{
		SourceProtocol: "qqofficial", TargetType: "group", TargetID: "G1",
	}); err != nil {
		t.Fatalf("SendReply: %v", err)
	}
	if len(sent) != 1 || sent[0] != "qqofficial-reply" {
		t.Fatalf("delivered via %v, want the originating adapter", sent)
	}
}

func TestAdapterRouterRefusesToGuessBetweenAdapters(t *testing.T) {
	t.Parallel()

	var sent []string
	router := NewRouter(map[string]ActionSender{
		"onebot11":    recordingSender{name: "onebot11", sent: &sent},
		"qq-official": recordingSender{name: "qqofficial", sent: &sent},
	}, map[string]string{"onebot11": "onebot11", "qq-official": "qqofficial"}, nil)

	// Target ids are namespaced per protocol, so picking one at random could
	// reach an unrelated conversation that happens to share an id.
	_, err := router.SendMessage(context.Background(), chatevent.OutboundMessageSend{
		TargetType: "group", TargetID: "G1",
	})
	if err == nil {
		t.Fatal("an ambiguous active push was delivered")
	}
	// Ambiguity is the caller's to resolve by naming an adapter, so it is a
	// send failure rather than an unavailable transport.
	var sendErr *chatevent.SendError
	if !errors.As(err, &sendErr) || sendErr.Code != errorcodes.AdapterSendFailed {
		t.Fatalf("error = %v, want code %s", err, errorcodes.AdapterSendFailed)
	}
	if len(sent) != 0 {
		t.Fatalf("delivered %v despite the ambiguity", sent)
	}

	if _, err := router.SendMessage(context.Background(), chatevent.OutboundMessageSend{
		SourceProtocol: "telegram", TargetType: "group", TargetID: "G1",
	}); err == nil {
		t.Fatal("a message for an unconnected adapter was delivered")
	}
}

func TestSendActionInheritsOnlyChatParentAdapter(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		origin         chatevent.Event
		sourceProtocol string
		wantAdapter    string
	}{
		{name: "message parent", origin: chatevent.Event{SourceAdapter: "qq-official", SourceProtocol: "qqofficial", EventType: "message.private"}, wantAdapter: "qq-official"},
		{name: "notice parent", origin: chatevent.Event{SourceAdapter: "qq-official", SourceProtocol: "qqofficial", EventType: "notice.friend_add"}, wantAdapter: "qq-official"},
		{name: "no parent"},
		{name: "platform parent", origin: chatevent.Event{SourceAdapter: "adapters.internal", SourceProtocol: "platform", EventType: "platform.bot_identities.changed"}},
		{name: "explicit protocol", origin: chatevent.Event{SourceAdapter: "qq-official", SourceProtocol: "qqofficial", EventType: "message.private"}, sourceProtocol: "onebot11", wantAdapter: "onebot11"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sent []string
			router := NewRouter(map[string]ActionSender{
				"onebot11":    recordingSender{name: "onebot11", sent: &sent},
				"qq-official": recordingSender{name: "qq-official", sent: &sent},
			}, map[string]string{"onebot11": "onebot11", "qq-official": "qqofficial"}, nil)
			result, err := SendAction(t.Context(), router, nil, tc.origin, chatevent.MessageCommand{
				Kind: "message.send", SourceProtocol: tc.sourceProtocol, TargetType: "private", TargetID: "U1",
			})
			if tc.wantAdapter == "" {
				var sendErr *chatevent.SendError
				if !errors.As(err, &sendErr) || sendErr.Code != errorcodes.AdapterSendFailed || len(sent) != 0 {
					t.Fatalf("ambiguous send: result=%+v, error=%v, sent=%v", result, err, sent)
				}
				return
			}
			if err != nil || result.SourceAdapter != tc.wantAdapter || len(sent) != 1 || sent[0] != tc.wantAdapter {
				t.Fatalf("send: result=%+v, error=%v, sent=%v, want adapter %s", result, err, sent, tc.wantAdapter)
			}
		})
	}
}

func TestAdapterRouterResolvesWhenOnlyOneAdapterIsConnected(t *testing.T) {
	t.Parallel()

	var sent []string
	router := NewRouter(map[string]ActionSender{
		"onebot11": recordingSender{name: "onebot11", sent: &sent},
	}, map[string]string{"onebot11": "onebot11"}, nil)
	// The common deployment: one adapter, so an active push is unambiguous.
	if _, err := router.SendMessage(context.Background(), chatevent.OutboundMessageSend{
		TargetType: "group", TargetID: "G1",
	}); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if len(sent) != 1 || sent[0] != "onebot11" {
		t.Fatalf("delivered via %v, want the only connected adapter", sent)
	}
}

type namingSender struct {
	recordingSender
	names map[string]string
}

func (s namingSender) ResolveTargetName(_ context.Context, adapterID, targetType, targetID string) string {
	return s.names[adapterID+"/"+targetType+":"+targetID]
}

type loginSender struct {
	recordingSender
	botID    string
	nickname string
}

func (s loginSender) ResolveBotDisplay(string) (string, string) {
	return s.botID, s.nickname
}

// Outbound log lines also name the account the message goes out as, which
// only the delivering adapter knows.
func TestAdapterRouterResolvesBotDisplayThroughTheOwningAdapter(t *testing.T) {
	t.Parallel()

	var sent []string
	router := NewRouter(map[string]ActionSender{
		"bot-one": loginSender{recordingSender{name: "bot-one", sent: &sent}, "10001", "测试机器人"},
		"bot-two": recordingSender{name: "bot-two", sent: &sent},
	}, map[string]string{"bot-one": "onebot11", "bot-two": "onebot11"}, nil)

	resolver, ok := any(router).(BotDisplayResolver)
	if !ok {
		t.Fatal("the router does not answer bot-display questions, so log lines lose the account name")
	}

	if id, nickname := resolver.ResolveBotDisplay("bot-one"); id != "10001" || nickname != "测试机器人" {
		t.Fatalf("ResolveBotDisplay = %q/%q, want the login of the owning adapter", id, nickname)
	}
	// A sender without login info, or one that is not connected, answers
	// nothing rather than borrowing the account of another adapter.
	if id, nickname := resolver.ResolveBotDisplay("bot-two"); id != "" || nickname != "" {
		t.Fatalf("ResolveBotDisplay = %q/%q, want no answer from a sender without login info", id, nickname)
	}
	if id, nickname := resolver.ResolveBotDisplay("no-such-bot"); id != "" || nickname != "" {
		t.Fatalf("ResolveBotDisplay = %q/%q, want no answer for an unknown adapter", id, nickname)
	}
}

// Outbound log lines name the conversation, and the router is what the pipeline
// holds. Without this the label falls back to a bare id for every adapter.
func TestAdapterRouterResolvesTargetNamesThroughTheOwningAdapter(t *testing.T) {
	t.Parallel()

	var sent []string
	router := NewRouter(map[string]ActionSender{
		"onebot11":   namingSender{recordingSender{name: "onebot11", sent: &sent}, map[string]string{"onebot11/group:200": "测试群"}},
		"second-bot": namingSender{recordingSender{name: "second-bot", sent: &sent}, map[string]string{"second-bot/group:200": "另一个群"}},
	}, map[string]string{"onebot11": "onebot11", "second-bot": "onebot11"}, nil)

	resolver, ok := any(router).(TargetDisplayResolver)
	if !ok {
		t.Fatal("the router does not answer target-name questions, so log lines lose their names")
	}

	// The same identifier means different conversations on different adapters,
	// so the answer has to come from the one that owns it.
	if got := resolver.ResolveTargetName(context.Background(), "onebot11", "group", "200"); got != "测试群" {
		t.Fatalf("ResolveTargetName = %q, want the owning adapter's name", got)
	}
	if got := resolver.ResolveTargetName(context.Background(), "second-bot", "group", "200"); got != "另一个群" {
		t.Fatalf("ResolveTargetName = %q, want the other adapter's name", got)
	}
	// An adapter that is not connected answers nothing rather than borrowing
	// another adapter's answer.
	if got := resolver.ResolveTargetName(context.Background(), "no-such-bot", "group", "200"); got != "" {
		t.Fatalf("ResolveTargetName = %q, want no answer for an unknown adapter", got)
	}
}
