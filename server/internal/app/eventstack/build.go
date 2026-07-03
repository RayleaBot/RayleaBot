package eventstack

import (
	"log/slog"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/eventingress"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
)

const dispatcherRuntimeFlushInterval = 10 * time.Second

type Deps struct {
	Config config.Config
	Logger *slog.Logger
}

type State struct {
	Adapter         *onebot11.Shell
	Bridge          *bridge.Bridge
	Dispatcher      *dispatch.Dispatcher
	ReplyTargets    *outbound.ReplyTargetCache
	OutboundSender  eventingress.OutboundActionSender
	OutboundLimiter *outbound.MessageRateLimiter
}

func Build(deps Deps) State {
	adapterShell := onebot11.New(deps.Config.OneBot, deps.Config.Adapter, deps.Logger)
	replyTargets := outbound.NewReplyTargetCache(outbound.DefaultReplyTargetCacheSize)
	eventDispatcher := dispatch.New(deps.Logger, adapterShell, replyTargets, deps.Config.Runtime.MaxPendingEventsPerPlugin)
	outboundLimiter := outbound.NewMessageRateLimiter(deps.Config)
	eventDispatcher.SetOutboundLimiter(outboundLimiter)
	eventBridge := bridge.New(deps.Logger, eventDispatcher)
	eventBridge.SetAdapterStatsSource(adapterShell)
	eventBridge.SetDispatcherStatsSource(metrics.NewDispatcherStatsAdapter(eventDispatcher))
	eventDispatcher.SetRuntimePublisher(metrics.NewDispatcherRuntimePublisher(eventBridge))
	eventDispatcher.StartObservabilityFlush(dispatcherRuntimeFlushInterval)

	return State{
		Adapter:         adapterShell,
		Bridge:          eventBridge,
		Dispatcher:      eventDispatcher,
		ReplyTargets:    replyTargets,
		OutboundSender:  adapterShell,
		OutboundLimiter: outboundLimiter,
	}
}

func (s *State) Close() {
	if s == nil || s.Dispatcher == nil {
		return
	}
	s.Dispatcher.Close()
	s.Dispatcher = nil
}
