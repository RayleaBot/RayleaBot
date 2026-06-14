package app

import (
	"context"
	"log/slog"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/adapter/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

type outboundActionSender interface {
	SendMessage(context.Context, adapteroutbound.OutboundMessageSend) (adapteroutbound.SendMessageResult, error)
	SendReply(context.Context, adapteroutbound.OutboundMessageReply) (adapteroutbound.SendMessageResult, error)
}

type eventIngressService struct {
	state            *appRuntimeState
	plugins          *plugincatalog.Catalog
	replyTargets     *outbound.ReplyTargetCache
	outboundSender   outboundActionSender
	outboundLimiter  outbound.MessageLimiter
	renderer         *renderservice.Service
	menu             *menuext.Service
	bridge           *bridge.Bridge
	lifecycle        *pluginservice.Controller
	metadataEnricher eventMetadataEnricher
	policy           *chatpolicy.Service
	whitelistRepo    permission.WhitelistRepository
	whitelistState   permission.WhitelistStateRepository
	blacklistRepo    permission.BlacklistRepository
}

func newEventIngressService(deps eventIngressDeps) *eventIngressService {
	service := &eventIngressService{
		state:            deps.state,
		plugins:          deps.plugins,
		replyTargets:     deps.replyTargets,
		outboundSender:   deps.outboundSender,
		outboundLimiter:  deps.outboundLimiter,
		renderer:         deps.renderer,
		menu:             deps.menu,
		bridge:           deps.bridge,
		lifecycle:        deps.lifecycle,
		metadataEnricher: deps.metadataEnricher,
		whitelistRepo:    deps.whitelistRepo,
		whitelistState:   deps.whitelistState,
		blacklistRepo:    deps.blacklistRepo,
	}
	service.policy = chatpolicy.New(chatpolicy.Deps{
		CurrentConfig: func() config.Config {
			if deps.state == nil {
				return config.Config{}
			}
			return deps.state.CurrentConfig()
		},
		Plugins:         deps.plugins,
		Menu:            deps.menu,
		Bridge:          deps.bridge,
		OutboundSender:  deps.outboundSender,
		OutboundLimiter: deps.outboundLimiter,
		Logger: func() *slog.Logger {
			if deps.state == nil {
				return nil
			}
			return deps.state.Logger
		}(),
		WhitelistRepo:  deps.whitelistRepo,
		WhitelistState: deps.whitelistState,
		BlacklistRepo:  deps.blacklistRepo,
	})
	return service
}

func (s *eventIngressService) UpdateConfig(cfg config.Config) {
	if s == nil {
		return
	}
	if s.policy != nil {
		s.policy.UpdateConfig(cfg)
	}
}

func (s *eventIngressService) applyChatPolicy(ctx context.Context, event adapterintake.NormalizedEvent) (adapterintake.NormalizedEvent, bool) {
	if s == nil || s.policy == nil {
		return event, true
	}
	s.policy.SetOutboundLimiter(s.outboundLimiter)
	return s.policy.Apply(ctx, event)
}

func (s *eventIngressService) enrichCommandEvent(event adapterintake.NormalizedEvent) adapterintake.NormalizedEvent {
	if s == nil || s.policy == nil {
		return event
	}
	return s.policy.EnrichCommandEvent(event)
}

func (s *eventIngressService) commandInfoForEvent(event adapterintake.NormalizedEvent) *permission.CommandInfo {
	if s == nil || s.policy == nil {
		return nil
	}
	return s.policy.CommandInfoForEvent(event)
}
