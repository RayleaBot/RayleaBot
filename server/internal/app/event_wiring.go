package app

import (
	"log/slog"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
)

const dispatcherRuntimeFlushInterval = 10 * time.Second

type eventDeps struct {
	Config         config.Config
	Logger         *slog.Logger
	BridgeDispatch bridge.Dispatch
}

type EventState struct {
	Adapter         *onebot11.Shell
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
	adapterShell := onebot11.New(deps.Config.OneBot, deps.Config.Adapter, deps.Logger)
	replyTargets := outbound.NewReplyTargetCache(outbound.DefaultReplyTargetCacheSize)
	eventDispatcher := dispatch.New(
		deps.Logger,
		adapterShell,
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
		Bridge:          eventBridge,
		Dispatcher:      eventDispatcher,
		ReplyTargets:    replyTargets,
		OutboundSender:  adapterShell,
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
