package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
)

// adapterRouter sends each outbound message through the adapter instance that
// owns the conversation. A reply carries the instance of the event it answers;
// an active push may carry only a protocol, or nothing at all, which resolves
// only while one candidate is connected.
type adapterRouter struct {
	senders   map[string]outbound.ActionSender
	protocols map[string]string
}

func newAdapterRouter(senders map[string]outbound.ActionSender, protocols map[string]string) *adapterRouter {
	return &adapterRouter{senders: senders, protocols: protocols}
}

func (r *adapterRouter) SendMessage(ctx context.Context, message chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error) {
	sender, err := r.resolve(message.SourceAdapter, message.SourceProtocol)
	if err != nil {
		return chatevent.SendMessageResult{}, err
	}
	return sender.SendMessage(ctx, message)
}

func (r *adapterRouter) SendReply(ctx context.Context, message chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error) {
	sender, err := r.resolve(message.SourceAdapter, message.SourceProtocol)
	if err != nil {
		return chatevent.SendMessageResult{}, err
	}
	return sender.SendReply(ctx, message)
}

// resolve refuses to guess. Target identifiers are namespaced per adapter, so
// delivering to the wrong one would either fail confusingly or reach an
// unrelated conversation that happens to share an id.
func (r *adapterRouter) resolve(sourceAdapter, sourceProtocol string) (outbound.ActionSender, error) {
	if adapterID := strings.TrimSpace(sourceAdapter); adapterID != "" {
		sender, ok := r.senders[adapterID]
		if !ok {
			return nil, fmt.Errorf("outbound: adapter %q is not connected", adapterID)
		}
		return sender, nil
	}

	// A caller that named only a protocol is answered when that protocol has
	// exactly one connected instance; with more than one, which conversation
	// the message belongs to is genuinely unknown.
	if protocol := strings.TrimSpace(sourceProtocol); protocol != "" {
		candidates := r.adaptersOfProtocol(protocol)
		switch len(candidates) {
		case 0:
			return nil, fmt.Errorf("outbound: no connected adapter serves protocol %q", protocol)
		case 1:
			return r.senders[candidates[0]], nil
		default:
			return nil, fmt.Errorf(
				"outbound: protocol %q has %d connected adapters (%s); name one or reply to an event so the adapter is known",
				protocol, len(candidates), strings.Join(candidates, ", "))
		}
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
		"outbound: message names no adapter and %d are connected (%s); reply to an event so the adapter is known",
		len(r.senders), strings.Join(r.adapterNames(), ", "),
	)
}

func (r *adapterRouter) adaptersOfProtocol(protocol string) []string {
	matched := make([]string, 0, len(r.senders))
	for id := range r.senders {
		if r.protocols[id] == protocol {
			matched = append(matched, id)
		}
	}
	sort.Strings(matched)
	return matched
}

func (r *adapterRouter) adapterNames() []string {
	names := make([]string, 0, len(r.senders))
	for name := range r.senders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
