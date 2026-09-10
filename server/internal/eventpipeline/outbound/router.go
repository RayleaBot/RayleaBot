package outbound

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

// Router sends each outbound message through the adapter instance that
// owns the conversation. A reply carries the instance of the event it answers;
// an active push may carry only a protocol, or nothing at all, which resolves
// only while one candidate is connected.
type Router struct {
	senders       map[string]ActionSender
	protocols     map[string]string
	currentConfig func() config.Config
}

// NewRouter binds a fixed adapter registry to an optional live configuration snapshot.
// A nil configuration source keeps every registered sender enabled.
func NewRouter(senders map[string]ActionSender, protocols map[string]string, currentConfig func() config.Config) *Router {
	return &Router{senders: senders, protocols: protocols, currentConfig: currentConfig}
}

func (r *Router) SendMessage(ctx context.Context, message chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error) {
	id, err := r.resolveID(message.SourceAdapter, message.SourceProtocol)
	if err != nil {
		return chatevent.SendMessageResult{}, err
	}
	return r.senders[id].SendMessage(ctx, message)
}

func (r *Router) SendReply(ctx context.Context, message chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error) {
	id, err := r.resolveID(message.SourceAdapter, message.SourceProtocol)
	if err != nil {
		return chatevent.SendMessageResult{}, err
	}
	return r.senders[id].SendReply(ctx, message)
}

// resolve refuses to guess. Target identifiers are namespaced per adapter, so
// delivering to the wrong one would either fail confusingly or reach an
// unrelated conversation that happens to share an id.
func (r *Router) resolveID(sourceAdapter, sourceProtocol string) (string, error) {
	senders := r.activeSenders()
	if adapterID := strings.TrimSpace(sourceAdapter); adapterID != "" {
		_, ok := senders[adapterID]
		if !ok {
			return "", fmt.Errorf("outbound: adapter %q is not connected", adapterID)
		}
		if protocol := strings.TrimSpace(sourceProtocol); protocol != "" && r.protocols[adapterID] != protocol {
			return "", fmt.Errorf("outbound: adapter %q does not serve protocol %q", adapterID, protocol)
		}
		return adapterID, nil
	}

	// A request with only a protocol can use exactly one connected instance.
	// If several instances are connected, the caller must name the adapter.
	if protocol := strings.TrimSpace(sourceProtocol); protocol != "" {
		candidates := r.adaptersOfProtocol(protocol, senders)
		switch len(candidates) {
		case 0:
			return "", fmt.Errorf("outbound: no connected adapter serves protocol %q", protocol)
		case 1:
			return candidates[0], nil
		default:
			return "", fmt.Errorf(
				"outbound: protocol %q has %d connected adapters (%s); name one or reply to an event so the adapter is known",
				protocol, len(candidates), strings.Join(candidates, ", "))
		}
	}

	if len(senders) == 1 {
		for adapterID := range senders {
			return adapterID, nil
		}
	}
	if len(senders) == 0 {
		return "", fmt.Errorf("outbound: no chat adapter is connected")
	}
	return "", fmt.Errorf(
		"outbound: message names no adapter and %d are connected (%s); reply to an event so the adapter is known",
		len(senders), strings.Join(r.adapterNames(senders), ", "),
	)
}

// ResolveTargetName forwards the question to the adapter the conversation
// belongs to. Without it the label would be built by whichever adapter the
// pipeline happened to hold, which for a keyed router is none of them.
func (r *Router) ResolveTargetName(ctx context.Context, adapterID, targetType, targetID string) string {
	sender, ok := r.activeSenders()[strings.TrimSpace(adapterID)]
	if !ok {
		return ""
	}
	resolver, ok := sender.(TargetDisplayResolver)
	if !ok {
		return ""
	}
	return resolver.ResolveTargetName(ctx, adapterID, targetType, targetID)
}

func (r *Router) adaptersOfProtocol(protocol string, senders map[string]ActionSender) []string {
	matched := make([]string, 0, len(senders))
	for id := range senders {
		if r.protocols[id] == protocol {
			matched = append(matched, id)
		}
	}
	sort.Strings(matched)
	return matched
}

func (r *Router) adapterNames(senders map[string]ActionSender) []string {
	names := make([]string, 0, len(senders))
	for name := range senders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *Router) activeSenders() map[string]ActionSender {
	if r.currentConfig == nil {
		return r.senders
	}
	cfg := r.currentConfig()
	active := make(map[string]ActionSender, len(r.senders))
	for _, instance := range cfg.Adapters {
		if sender, ok := r.senders[instance.ID]; ok && instance.Enabled && instance.Type == r.protocols[instance.ID] {
			active[instance.ID] = sender
		}
	}
	return active
}

// ResolveScope uses the same adapter selection as delivery; unknown or
// ambiguous routes remain isolated from valid target quotas.
func (r *Router) ResolveScope(scope chatevent.IdentityScope, identities []chatevent.BotIdentity) chatevent.IdentityScope {
	if scope.BotID != "" {
		return scope
	}
	id, err := r.resolveID(scope.SourceAdapter, scope.SourceProtocol)
	if err != nil {
		return scope
	}
	scope.Kind, scope.SourceAdapter, scope.SourceProtocol = "instance", id, r.protocols[id]
	for _, identity := range identities {
		if identity.SourceAdapter == id && identity.SourceProtocol == scope.SourceProtocol {
			scope.BotID = identity.ID
			break
		}
	}
	return scope
}
