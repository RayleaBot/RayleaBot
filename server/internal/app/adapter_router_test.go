package app

import (
	"context"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
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
	router := newAdapterRouter(map[string]outbound.ActionSender{
		"onebot11":   recordingSender{name: "onebot11", sent: &sent},
		"qqofficial": recordingSender{name: "qqofficial", sent: &sent},
	})

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
	router := newAdapterRouter(map[string]outbound.ActionSender{
		"onebot11":   recordingSender{name: "onebot11", sent: &sent},
		"qqofficial": recordingSender{name: "qqofficial", sent: &sent},
	})

	// Target ids are namespaced per protocol, so picking one at random could
	// reach an unrelated conversation that happens to share an id.
	_, err := router.SendMessage(context.Background(), chatevent.OutboundMessageSend{
		TargetType: "group", TargetID: "G1",
	})
	if err == nil {
		t.Fatal("an ambiguous active push was delivered")
	}
	if !strings.Contains(err.Error(), "onebot11") || !strings.Contains(err.Error(), "qqofficial") {
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
	router := newAdapterRouter(map[string]outbound.ActionSender{
		"onebot11": recordingSender{name: "onebot11", sent: &sent},
	})
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
