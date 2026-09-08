package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
)

// adapterRouter sends each outbound message through the adapter instance that
// owns the conversation. A reply carries the instance of the event it answers;
// an active push may carry only a protocol, or nothing at all, which resolves
// only while one candidate is connected.
type adapterRouter struct {
	senders       map[string]outbound.ActionSender
	protocols     map[string]string
	currentConfig func() config.Config
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
	senders := r.activeSenders()
	if adapterID := strings.TrimSpace(sourceAdapter); adapterID != "" {
		sender, ok := senders[adapterID]
		if !ok {
			return nil, fmt.Errorf("outbound: adapter %q is not connected", adapterID)
		}
		if protocol := strings.TrimSpace(sourceProtocol); protocol != "" && r.protocols[adapterID] != protocol {
			return nil, fmt.Errorf("outbound: adapter %q does not serve protocol %q", adapterID, protocol)
		}
		return sender, nil
	}

	// A request with only a protocol can use exactly one connected instance.
	// If several instances are connected, the caller must name the adapter.
	if protocol := strings.TrimSpace(sourceProtocol); protocol != "" {
		candidates := r.adaptersOfProtocol(protocol, senders)
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

	if len(senders) == 1 {
		for _, sender := range senders {
			return sender, nil
		}
	}
	if len(senders) == 0 {
		return nil, fmt.Errorf("outbound: no chat adapter is connected")
	}
	return nil, fmt.Errorf(
		"outbound: message names no adapter and %d are connected (%s); reply to an event so the adapter is known",
		len(senders), strings.Join(r.adapterNames(senders), ", "),
	)
}

// ResolveTargetName forwards the question to the adapter the conversation
// belongs to. Without it the label would be built by whichever adapter the
// pipeline happened to hold, which for a keyed router is none of them.
func (r *adapterRouter) ResolveTargetName(ctx context.Context, adapterID, targetType, targetID string) string {
	sender, ok := r.activeSenders()[strings.TrimSpace(adapterID)]
	if !ok {
		return ""
	}
	resolver, ok := sender.(outbound.TargetDisplayResolver)
	if !ok {
		return ""
	}
	return resolver.ResolveTargetName(ctx, adapterID, targetType, targetID)
}

func (r *adapterRouter) adaptersOfProtocol(protocol string, senders map[string]outbound.ActionSender) []string {
	matched := make([]string, 0, len(senders))
	for id := range senders {
		if r.protocols[id] == protocol {
			matched = append(matched, id)
		}
	}
	sort.Strings(matched)
	return matched
}

func (r *adapterRouter) adapterNames(senders map[string]outbound.ActionSender) []string {
	names := make([]string, 0, len(senders))
	for name := range senders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *adapterRouter) activeSenders() map[string]outbound.ActionSender {
	if r.currentConfig == nil {
		return r.senders
	}
	cfg := r.currentConfig()
	active := make(map[string]outbound.ActionSender, len(r.senders))
	for _, instance := range cfg.Adapters {
		if sender, ok := r.senders[instance.ID]; ok && instance.Enabled && instance.Type == r.protocols[instance.ID] {
			active[instance.ID] = sender
		}
	}
	return active
}
