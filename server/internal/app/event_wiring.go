package app

import (
	"context"
	"log/slog"
	"time"

	adapterservice "github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/qqofficial"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/conversation"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

const dispatcherRuntimeFlushInterval = 10 * time.Second

type eventDeps struct {
	MessageSent    func(string, string)
	Config         config.Config
	Logger         *slog.Logger
	BridgeDispatch bridge.Dispatch
}

type EventState struct {
	MessageSent     func(string, string)
	Adapters        *adapterservice.Registry
	BotIdentity     botIdentitySource
	Bridge          *bridge.Bridge
	Dispatcher      *dispatch.Dispatcher
	Conversations   *conversation.Registry
	ReplyTargets    *outbound.ReplyTargetCache
	OutboundSender  outbound.ActionSender
	AdapterRouter   *outbound.Router
	OutboundLimiter *outbound.MessagePolicy
}

func buildEvents(deps eventDeps) EventState {
	if deps.MessageSent == nil {
		deps.MessageSent = func(string, string) {}
	}
	// Adapters are built from the configured instances and keyed by instance id.
	// Several instances may share a protocol, so routing keys on the id.
	oneBotShells := make(map[string]*onebot11.Shell, 1)
	qqClients := make(map[string]adapterservice.QQOfficialAdapter, 1)

	for _, instance := range deps.Config.Adapters {
		switch {
		case instance.Type == config.AdapterTypeOneBot11 && instance.OneBot11 != nil:
			// The shell is built either way so the management surface can show
			// and edit the transports of an adapter the operator switched off.
			settings, _ := deps.Config.OneBot11RuntimeSettings(instance.ID)
			shell := onebot11.New(instance.ID, settings, deps.Config.Adapter, deps.Logger)
			oneBotShells[instance.ID] = shell
		case instance.Type == config.AdapterTypeQQOfficial && instance.QQOfficial != nil:
			client := qqofficial.New(instance.ID, *instance.QQOfficial, deps.Config.Adapter, deps.Logger)
			client.SetEnabled(instance.Enabled)
			qqClients[instance.ID] = client
		}
	}

	registry := adapterservice.NewRegistry(deps.Config, oneBotShells, qqClients)
	identity := botIdentitySource{snapshot: func() (config.Config, map[string]botIdentityProvider) {
		snapshot := registry.Snapshot()
		providers := make(map[string]botIdentityProvider)
		for _, instance := range snapshot.Config().Adapters {
			if shell := snapshot.OneBot11(instance.ID); shell != nil {
				providers[instance.ID] = botIdentityProvider{protocol: config.AdapterTypeOneBot11, identity: func() chatevent.BotIdentity {
					return chatevent.BotIdentity{SourceAdapter: instance.ID, SourceProtocol: config.AdapterTypeOneBot11, ID: shell.CurrentBotID()}
				}}
			} else if client := snapshot.QQOfficial(instance.ID); client != nil {
				providers[instance.ID] = botIdentityProvider{protocol: config.AdapterTypeQQOfficial, identity: func() chatevent.BotIdentity {
					status := client.Status()
					return chatevent.BotIdentity{SourceAdapter: instance.ID, SourceProtocol: config.AdapterTypeQQOfficial, ID: status.BotID, Nickname: status.BotName}
				}}
			}
		}
		return snapshot.Config(), providers
	}}
	outboundSender := outbound.NewLiveRouter(func() outbound.Routes { return adapterRoutes(registry.Snapshot()) }, deps.MessageSent)

	replyTargets := outbound.NewReplyTargetCache(outbound.DefaultReplyTargetCacheSize)
	eventDispatcher := dispatch.New(
		deps.Logger,
		outboundSender,
		replyTargets,
		deps.Config.Runtime.MaxPendingEventsPerPlugin,
		deps.Config.Runtime.MaxPendingControlEvents,
	)
	conversationRegistry := conversation.New(conversation.Options{NotifyExpired: func(owner conversation.Owner, event chatevent.Event) {
		eventDispatcher.DispatchToProcess(context.Background(), owner.PluginID, owner.Done, event)
	}})
	outboundPolicy := outbound.NewMessagePolicy(deps.Config, func(scope chatevent.IdentityScope) chatevent.IdentityScope {
		if scope.BotID != "" {
			return scope
		}
		return outboundSender.ResolveScope(scope, identity.BotIdentities())
	})
	eventDispatcher.SetOutboundPolicy(outboundPolicy)
	var bridgeDispatch bridge.Dispatch = eventDispatcher
	if deps.BridgeDispatch != nil {
		bridgeDispatch = deps.BridgeDispatch
	}
	eventBridge := bridge.New(deps.Logger, bridgeDispatch)
	eventBridge.SetAdapterStatsSource(registry)
	eventBridge.SetDispatcherStatsSource(NewDispatcherStatsAdapter(eventDispatcher))
	eventDispatcher.SetRuntimePublisher(NewDispatcherRuntimePublisher(eventBridge))
	eventDispatcher.StartObservabilityFlush(dispatcherRuntimeFlushInterval)

	return EventState{
		MessageSent:     deps.MessageSent,
		Adapters:        registry,
		BotIdentity:     identity,
		Bridge:          eventBridge,
		Dispatcher:      eventDispatcher,
		Conversations:   conversationRegistry,
		ReplyTargets:    replyTargets,
		OutboundSender:  outboundSender,
		AdapterRouter:   outboundSender,
		OutboundLimiter: outboundPolicy,
	}
}

func (s *EventState) Close() {
	if s.Conversations != nil {
		s.Conversations.Close()
		s.Conversations = nil
	}
	if s.Dispatcher == nil {
		return
	}
	s.Dispatcher.Close()
	s.Dispatcher = nil
}

func adapterRoutes(snapshot *adapterservice.RuntimeSnapshot) outbound.Routes {
	cfg := snapshot.Config()
	routes := outbound.Routes{Config: &cfg, Senders: make(map[string]outbound.ActionSender), Protocols: make(map[string]string)}
	for _, instance := range cfg.Adapters {
		if shell := snapshot.OneBot11(instance.ID); shell != nil {
			routes.Senders[instance.ID] = shell
			routes.Protocols[instance.ID] = config.AdapterTypeOneBot11
		} else if client, ok := snapshot.QQOfficial(instance.ID).(outbound.ActionSender); ok {
			routes.Senders[instance.ID] = client
			routes.Protocols[instance.ID] = config.AdapterTypeQQOfficial
		}
	}
	return routes
}
