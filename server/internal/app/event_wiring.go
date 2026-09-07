package app

import (
	"log/slog"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/qqofficial"
)

const dispatcherRuntimeFlushInterval = 10 * time.Second

type eventDeps struct {
	Config         config.Config
	Logger         *slog.Logger
	BridgeDispatch bridge.Dispatch
}

type EventState struct {
	Adapter         *onebot11.Shell
	OneBotShells    map[string]*onebot11.Shell
	QQOfficial      map[string]*qqofficial.Client
	Bridge          *bridge.Bridge
	Dispatcher      *dispatch.Dispatcher
	ReplyTargets    *outbound.ReplyTargetCache
	OutboundSender  outbound.ActionSender
	OutboundLimiter outboundRuntimePolicy
	OutboundPolicy  *outbound.MessagePolicy
}

type outboundRuntimePolicy interface {
	outbound.MessageLimiter
	ApplyConfig(config.Config)
}

func buildEvents(deps eventDeps) EventState {
	// Adapters are built from the configured instances and keyed by instance id.
	// Several instances may share a protocol, so routing keys on the id.
	senders := make(map[string]outbound.ActionSender, len(deps.Config.Adapters))
	protocols := make(map[string]string, len(deps.Config.Adapters))
	oneBotShells := make(map[string]*onebot11.Shell, 1)
	qqClients := make(map[string]*qqofficial.Client, 1)

	for _, instance := range deps.Config.Adapters {
		if !instance.Enabled {
			// A disabled instance is a choice, not a failure: nothing is built
			// for it, so it neither connects nor accepts inbound traffic.
			continue
		}
		switch {
		case instance.Type == config.AdapterTypeOneBot11 && instance.OneBot11 != nil:
			shell := onebot11.New(instance.ID, *instance.OneBot11, deps.Config.Adapter, deps.Logger)
			oneBotShells[instance.ID] = shell
			senders[instance.ID] = shell
			protocols[instance.ID] = instance.Type
		case instance.Type == config.AdapterTypeQQOfficial && instance.QQOfficial != nil:
			client := qqofficial.New(instance.ID, *instance.QQOfficial, deps.Config.Adapter, deps.Logger)
			qqClients[instance.ID] = client
			senders[instance.ID] = client
			protocols[instance.ID] = instance.Type
		}
	}

	// The dispatcher and the OneBot protocol surface still speak about a single
	// OneBot connection; that is the first configured one.
	var adapterShell *onebot11.Shell
	if instance, _, ok := deps.Config.PrimaryOneBot11(); ok {
		adapterShell = oneBotShells[instance.ID]
	}
	if adapterShell == nil {
		adapterShell = onebot11.New(config.DefaultOneBot11AdapterID, config.OneBotConfig{}, deps.Config.Adapter, deps.Logger)
	}
	outboundSender := newAdapterRouter(senders, protocols)

	replyTargets := outbound.NewReplyTargetCache(outbound.DefaultReplyTargetCacheSize)
	eventDispatcher := dispatch.New(
		deps.Logger,
		outboundSender,
		replyTargets,
		deps.Config.Runtime.MaxPendingEventsPerPlugin,
		deps.Config.Runtime.MaxPendingControlEvents,
	)
	outboundPolicy := outbound.NewMessagePolicy(deps.Config)
	eventDispatcher.SetOutboundLimiter(outboundPolicy)
	eventDispatcher.SetOutboundCircuitBreaker(outboundPolicy.Breaker)
	var bridgeDispatch bridge.Dispatch = eventDispatcher
	if deps.BridgeDispatch != nil {
		bridgeDispatch = deps.BridgeDispatch
	}
	eventBridge := bridge.New(deps.Logger, bridgeDispatch)
	eventBridge.SetAdapterStatsSource(adapterShell)
	eventBridge.SetDispatcherStatsSource(NewDispatcherStatsAdapter(eventDispatcher))
	eventDispatcher.SetRuntimePublisher(NewDispatcherRuntimePublisher(eventBridge))
	eventDispatcher.StartObservabilityFlush(dispatcherRuntimeFlushInterval)

	return EventState{
		Adapter:         adapterShell,
		OneBotShells:    oneBotShells,
		QQOfficial:      qqClients,
		Bridge:          eventBridge,
		Dispatcher:      eventDispatcher,
		ReplyTargets:    replyTargets,
		OutboundSender:  outboundSender,
		OutboundLimiter: outboundPolicy,
		OutboundPolicy:  outboundPolicy,
	}
}

func (s *EventState) Close() {
	if s.Dispatcher == nil {
		return
	}
	s.Dispatcher.Close()
	s.Dispatcher = nil
}
