package app

import (
	"log/slog"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/qqofficial"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

const dispatcherRuntimeFlushInterval = 10 * time.Second

type eventDeps struct {
	Config         config.Config
	CurrentConfig  func() config.Config
	Logger         *slog.Logger
	BridgeDispatch bridge.Dispatch
}

type EventState struct {
	// Adapter objects live for the application lifetime. Their instance switch
	// gates transports and routing, so toggling it does not mutate these maps.
	OneBotShells    map[string]*onebot11.Shell
	QQOfficial      map[string]*qqofficial.Client
	BotIdentity     botIdentitySource
	Bridge          *bridge.Bridge
	Dispatcher      *dispatch.Dispatcher
	ReplyTargets    *outbound.ReplyTargetCache
	OutboundSender  outbound.ActionSender
	AdapterRouter   *outbound.Router
	OutboundLimiter *outbound.MessagePolicy
}

func buildEvents(deps eventDeps) EventState {
	// Adapters are built from the configured instances and keyed by instance id.
	// Several instances may share a protocol, so routing keys on the id.
	senders := make(map[string]outbound.ActionSender, len(deps.Config.Adapters))
	protocols := make(map[string]string, len(deps.Config.Adapters))
	oneBotShells := make(map[string]*onebot11.Shell, 1)
	qqClients := make(map[string]*qqofficial.Client, 1)
	currentConfig := deps.CurrentConfig
	if currentConfig == nil {
		currentConfig = func() config.Config { return deps.Config }
	}
	isEnabled := func(id, protocol string) bool {
		instance, ok := currentConfig().AdapterByID(id)
		return ok && instance.Enabled && instance.Type == protocol
	}
	// Providers are appended in configuration order, which is the order the
	// identity source falls back through.
	var identity botIdentitySource

	for _, instance := range deps.Config.Adapters {
		switch {
		case instance.Type == config.AdapterTypeOneBot11 && instance.OneBot11 != nil:
			// The shell is built either way so the management surface can show
			// and edit the transports of an adapter the operator switched off.
			settings, _ := deps.Config.OneBot11RuntimeSettings(instance.ID)
			shell := onebot11.New(instance.ID, settings, deps.Config.Adapter, deps.Logger)
			oneBotShells[instance.ID] = shell
			senders[instance.ID] = shell
			protocols[instance.ID] = instance.Type
			identity.providers = append(identity.providers, func() chatevent.BotIdentity {
				if !isEnabled(instance.ID, instance.Type) {
					return chatevent.BotIdentity{}
				}
				return chatevent.BotIdentity{SourceAdapter: instance.ID, SourceProtocol: instance.Type, ID: shell.CurrentBotID()}
			})
		case instance.Type == config.AdapterTypeQQOfficial && instance.QQOfficial != nil:
			client := qqofficial.New(instance.ID, *instance.QQOfficial, deps.Config.Adapter, deps.Logger)
			client.SetEnabled(instance.Enabled)
			qqClients[instance.ID] = client
			senders[instance.ID] = client
			protocols[instance.ID] = instance.Type
			identity.providers = append(identity.providers, func() chatevent.BotIdentity {
				if !isEnabled(instance.ID, instance.Type) {
					return chatevent.BotIdentity{}
				}
				botID, nickname := client.BotIdentity()
				return chatevent.BotIdentity{SourceAdapter: instance.ID, SourceProtocol: instance.Type, ID: botID, Nickname: nickname}
			})
		}
	}

	outboundSender := outbound.NewRouter(senders, protocols, currentConfig)

	replyTargets := outbound.NewReplyTargetCache(outbound.DefaultReplyTargetCacheSize)
	eventDispatcher := dispatch.New(
		deps.Logger,
		outboundSender,
		replyTargets,
		deps.Config.Runtime.MaxPendingEventsPerPlugin,
		deps.Config.Runtime.MaxPendingControlEvents,
	)
	outboundPolicy := outbound.NewMessagePolicy(deps.Config, func(scope chatevent.IdentityScope) chatevent.IdentityScope {
		return outboundSender.ResolveScope(scope, identity.BotIdentities())
	})
	eventDispatcher.SetOutboundPolicy(outboundPolicy)
	var bridgeDispatch bridge.Dispatch = eventDispatcher
	if deps.BridgeDispatch != nil {
		bridgeDispatch = deps.BridgeDispatch
	}
	eventBridge := bridge.New(deps.Logger, bridgeDispatch)
	eventBridge.SetAdapterStatsSource(EventState{OneBotShells: oneBotShells})
	eventBridge.SetDispatcherStatsSource(NewDispatcherStatsAdapter(eventDispatcher))
	eventDispatcher.SetRuntimePublisher(NewDispatcherRuntimePublisher(eventBridge))
	eventDispatcher.StartObservabilityFlush(dispatcherRuntimeFlushInterval)

	return EventState{
		OneBotShells:    oneBotShells,
		BotIdentity:     identity,
		QQOfficial:      qqClients,
		Bridge:          eventBridge,
		Dispatcher:      eventDispatcher,
		ReplyTargets:    replyTargets,
		OutboundSender:  outboundSender,
		AdapterRouter:   outboundSender,
		OutboundLimiter: outboundPolicy,
	}
}

func (s *EventState) Close() {
	if s.Dispatcher == nil {
		return
	}
	s.Dispatcher.Close()
	s.Dispatcher = nil
}
