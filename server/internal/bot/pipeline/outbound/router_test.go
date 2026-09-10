package outbound

import (
	"context"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
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
	// The error names the instances, because naming one is how the caller
	// resolves the ambiguity.
	if !strings.Contains(err.Error(), "onebot11") || !strings.Contains(err.Error(), "qq-official") {
		t.Fatalf("error = %v, want it to name the connected adapters", err)
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

func TestAdapterRouterHonoursAPluginNamedProtocol(t *testing.T) {
	t.Parallel()

	var sent []string
	router := NewRouter(map[string]ActionSender{
		"onebot11":    recordingSender{name: "onebot11", sent: &sent},
		"qq-official": recordingSender{name: "qqofficial", sent: &sent},
	}, map[string]string{"onebot11": "onebot11", "qq-official": "qqofficial"}, nil)

	// With two adapters connected an active push is otherwise ambiguous; naming
	// the protocol is how a plugin resolves it.
	if _, err := router.SendMessage(context.Background(), chatevent.OutboundMessageSend{
		SourceProtocol: "qqofficial", TargetType: "group", TargetID: "G1",
	}); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if len(sent) != 1 || sent[0] != "qqofficial" {
		t.Fatalf("delivered via %v, want the named adapter", sent)
	}
}

type namingSender struct {
	recordingSender
	names map[string]string
}

func (s namingSender) ResolveTargetName(_ context.Context, adapterID, targetType, targetID string) string {
	return s.names[adapterID+"/"+targetType+":"+targetID]
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
