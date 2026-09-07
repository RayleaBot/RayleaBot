package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
)

// adapterRouter sends each outbound message through the adapter that owns the
// conversation. A reply carries the protocol of the event it answers; an active
// push carries none, which only resolves while a single adapter is connected.
type adapterRouter struct {
	senders map[string]outbound.ActionSender
}

func newAdapterRouter(senders map[string]outbound.ActionSender) *adapterRouter {
	return &adapterRouter{senders: senders}
}

func (r *adapterRouter) SendMessage(ctx context.Context, message chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error) {
	sender, err := r.resolve(message.SourceProtocol)
	if err != nil {
		return chatevent.SendMessageResult{}, err
	}
	return sender.SendMessage(ctx, message)
}

func (r *adapterRouter) SendReply(ctx context.Context, message chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error) {
	sender, err := r.resolve(message.SourceProtocol)
	if err != nil {
		return chatevent.SendMessageResult{}, err
	}
	return sender.SendReply(ctx, message)
}

// resolve refuses to guess. Target identifiers are namespaced per protocol, so
// delivering to the wrong adapter would either fail confusingly or reach an
// unrelated conversation that happens to share an id.
func (r *adapterRouter) resolve(sourceProtocol string) (outbound.ActionSender, error) {
	protocol := strings.TrimSpace(sourceProtocol)
	if protocol != "" {
		sender, ok := r.senders[protocol]
		if !ok {
			return nil, fmt.Errorf("outbound: no connected adapter serves protocol %q", protocol)
		}
		return sender, nil
	}
	if len(r.senders) == 1 {
		for _, sender := range r.senders {
			return sender, nil
		}
	}
	if len(r.senders) == 0 {
		return nil, fmt.Errorf("outbound: no chat adapter is connected")
	}
	return nil, fmt.Errorf(
		"outbound: message names no protocol and %d adapters are connected (%s); reply to an event so the adapter is known",
		len(r.senders), strings.Join(r.protocolNames(), ", "),
	)
}

func (r *adapterRouter) protocolNames() []string {
	names := make([]string, 0, len(r.senders))
	for name := range r.senders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
