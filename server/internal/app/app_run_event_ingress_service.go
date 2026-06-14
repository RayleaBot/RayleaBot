package app

import (
	"context"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/command"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

type outboundActionSender interface {
	SendMessage(context.Context, adapter.OutboundMessageSend) (adapter.SendMessageResult, error)
	SendReply(context.Context, adapter.OutboundMessageReply) (adapter.SendMessageResult, error)
}

type eventIngressService struct {
	state             *appRuntimeState
	plugins           *plugincatalog.Catalog
	replyTargets      *replyTargetCache
	outboundSender    outboundActionSender
	outboundLimiter   outbound.MessageLimiter
	renderer          *render.Service
	menu              *menuext.Service
	bridge            *bridge.Bridge
	lifecycle         *pluginservice.Controller
	metadataEnricher  eventMetadataEnricher
	commandParser     *command.Parser
	permissionChecker *permission.Checker
	whitelistRepo     permission.WhitelistRepository
	whitelistState    permission.WhitelistStateRepository
	blacklistRepo     permission.BlacklistRepository
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
	service.UpdateConfig(deps.state.Config)
	return service
}

func (s *eventIngressService) UpdateConfig(cfg config.Config) {
	if s == nil {
		return
	}
	s.commandParser = newCommandParser(cfg)
	s.permissionChecker = newPermissionChecker(cfg, s.whitelistRepo, s.whitelistState, s.blacklistRepo)
}
