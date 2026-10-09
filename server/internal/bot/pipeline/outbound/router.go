package outbound

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

type routingTable struct {
	messageSent   func(string, string)
	senders       map[string]ActionSender
	protocols     map[string]string
	currentConfig func() config.Config
}

// Routes is a consistent membership and configuration snapshot for one operation.
type Routes struct {
	Senders   map[string]ActionSender
	Protocols map[string]string
	Config    *config.Config
}

// Router sends each outbound message through the adapter instance that
// owns the conversation. A reply carries the instance of the event it answers;
// an active push may carry only a protocol, or nothing at all, which resolves
// only while one candidate is connected.
type Router struct {
	snapshot func() *routingTable
}

// NewRouter binds fixed senders to an optional live configuration source.
func NewRouter(senders map[string]ActionSender, protocols map[string]string, currentConfig func() config.Config, observers ...func(string, string)) *Router {
	senders, protocols = maps.Clone(senders), maps.Clone(protocols)
	return NewLiveRouter(func() Routes {
		routes := Routes{Senders: senders, Protocols: protocols}
		if currentConfig != nil {
			cfg := currentConfig()
			routes.Config = &cfg
		}
		return routes
	}, observers...)
}

func NewLiveRouter(source func() Routes, observers ...func(string, string)) *Router {
	messageSent := func(string, string) {}
	if len(observers) > 0 && observers[0] != nil {
		messageSent = observers[0]
	}
	return &Router{snapshot: func() *routingTable {
		routes := source()
		table := &routingTable{senders: routes.Senders, protocols: routes.Protocols, messageSent: messageSent}
		if routes.Config != nil {
			table.currentConfig = func() config.Config { return *routes.Config }
		}
		return table
	}}
}

func (r *Router) SendMessage(ctx context.Context, message chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error) {
	return r.snapshot().SendMessage(ctx, message)
}
func (r *Router) SendReply(ctx context.Context, message chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error) {
	return r.snapshot().SendReply(ctx, message)
}
func (r *Router) ResolveAdapterID(adapter, protocol string) (string, error) {
	return r.snapshot().ResolveAdapterID(adapter, protocol)
}
func (r *Router) ResolveTargetName(ctx context.Context, adapter, targetType, targetID string) string {
	return r.snapshot().ResolveTargetName(ctx, adapter, targetType, targetID)
}
func (r *Router) ResolveBotDisplay(adapter string) (string, string) {
	return r.snapshot().ResolveBotDisplay(adapter)
}
func (r *Router) ResolveScope(scope chatevent.IdentityScope, identities []chatevent.BotIdentity) chatevent.IdentityScope {
	return r.snapshot().ResolveScope(scope, identities)
}

func (r *routingTable) SendMessage(ctx context.Context, message chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error) {
	id, err := r.ResolveAdapterID(message.SourceAdapter, message.SourceProtocol)
	if err != nil {
		return chatevent.SendMessageResult{SourceAdapter: message.SourceAdapter, SourceProtocol: message.SourceProtocol}, err
	}
	message.SourceAdapter, message.SourceProtocol = id, r.protocols[id]
	result, err := r.senders[id].SendMessage(ctx, message)
	if err == nil {
		r.messageSent(id, r.protocols[id])
	}
	result.SourceAdapter, result.SourceProtocol = id, r.protocols[id]
	return result, err
}

func (r *routingTable) SendReply(ctx context.Context, message chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error) {
	id, err := r.ResolveAdapterID(message.SourceAdapter, message.SourceProtocol)
	if err != nil {
		return chatevent.SendMessageResult{SourceAdapter: message.SourceAdapter, SourceProtocol: message.SourceProtocol}, err
	}
	message.SourceAdapter, message.SourceProtocol = id, r.protocols[id]
	result, err := r.senders[id].SendReply(ctx, message)
	if err == nil {
		r.messageSent(id, r.protocols[id])
	}
	result.SourceAdapter, result.SourceProtocol = id, r.protocols[id]
	return result, err
}

// ResolveAdapterID selects one enabled instance from the live configuration.
// Target identifiers are namespaced per adapter, so
// delivering to the wrong one would either fail confusingly or reach an
// unrelated conversation that happens to share an id.
func (r *routingTable) ResolveAdapterID(sourceAdapter, sourceProtocol string) (string, error) {
	if adapterID := strings.TrimSpace(sourceAdapter); adapterID != "" {
		_, ok := r.activeSender(adapterID)
		if !ok {
			return "", routeFailure(errorcodes.AdapterTransportUnavailable, "outbound: adapter %q is not connected", adapterID)
		}
		if protocol := strings.TrimSpace(sourceProtocol); protocol != "" && r.protocols[adapterID] != protocol {
			return "", routeFailure(errorcodes.AdapterSendFailed, "outbound: adapter %q does not serve protocol %q", adapterID, protocol)
		}
		return adapterID, nil
	}
	senders := r.activeSenders()

	// A request with only a protocol can use exactly one connected instance.
	// If several instances are connected, the caller must name the adapter.
	if protocol := strings.TrimSpace(sourceProtocol); protocol != "" {
		candidates := r.adaptersOfProtocol(protocol, senders)
		switch len(candidates) {
		case 0:
			return "", routeFailure(errorcodes.AdapterTransportUnavailable, "outbound: no connected adapter serves protocol %q", protocol)
		case 1:
			return candidates[0], nil
		default:
			return "", routeFailure(errorcodes.AdapterSendFailed,
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
		return "", routeFailure(errorcodes.AdapterTransportUnavailable, "outbound: no chat adapter is connected")
	}
	return "", routeFailure(errorcodes.AdapterSendFailed,
		"outbound: message names no adapter and %d are connected (%s); reply to an event so the adapter is known",
		len(senders), strings.Join(r.adapterNames(senders), ", "),
	)
}

// ResolveTargetName forwards the question to the adapter the conversation
// belongs to. Without it the label would be built by whichever adapter the
// pipeline happened to hold, which for a keyed router is none of them.
func (r *routingTable) ResolveTargetName(ctx context.Context, adapterID, targetType, targetID string) string {
	sender, ok := r.activeSender(strings.TrimSpace(adapterID))
	if !ok {
		return ""
	}
	resolver, ok := sender.(TargetDisplayResolver)
	if !ok {
		return ""
	}
	return resolver.ResolveTargetName(ctx, adapterID, targetType, targetID)
}

// ResolveBotDisplay forwards the question to the named adapter for the same
// reason as ResolveTargetName: the router itself is signed in as nobody.
func (r *routingTable) ResolveBotDisplay(adapterID string) (string, string) {
	sender, ok := r.activeSender(strings.TrimSpace(adapterID))
	if !ok {
		return "", ""
	}
	resolver, ok := sender.(BotDisplayResolver)
	if !ok {
		return "", ""
	}
	return resolver.ResolveBotDisplay(adapterID)
}

func (r *routingTable) adaptersOfProtocol(protocol string, senders map[string]ActionSender) []string {
	matched := make([]string, 0, len(senders))
	for id := range senders {
		if r.protocols[id] == protocol {
			matched = append(matched, id)
		}
	}
	sort.Strings(matched)
	return matched
}

func (r *routingTable) adapterNames(senders map[string]ActionSender) []string {
	names := make([]string, 0, len(senders))
	for name := range senders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *routingTable) activeSender(id string) (ActionSender, bool) {
	sender, ok := r.senders[id]
	if !ok || r.currentConfig == nil {
		return sender, ok
	}
	for _, instance := range r.currentConfig().Adapters {
		if instance.ID == id && instance.Enabled && instance.Type == r.protocols[id] {
			return sender, true
		}
	}
	return nil, false
}

func (r *routingTable) activeSenders() map[string]ActionSender {
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
func (r *routingTable) ResolveScope(scope chatevent.IdentityScope, identities []chatevent.BotIdentity) chatevent.IdentityScope {
	if scope.BotID != "" {
		return scope
	}
	id, err := r.ResolveAdapterID(scope.SourceAdapter, scope.SourceProtocol)
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

func routeFailure(code, format string, args ...any) error {
	return &chatevent.SendError{Code: code, Message: fmt.Sprintf(format, args...)}
}
