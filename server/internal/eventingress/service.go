package eventingress

import (
	"context"
	"log/slog"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

type OutboundActionSender interface {
	SendMessage(context.Context, adapteroutbound.OutboundMessageSend) (adapteroutbound.SendMessageResult, error)
	SendReply(context.Context, adapteroutbound.OutboundMessageReply) (adapteroutbound.SendMessageResult, error)
}

type MetadataEnricher interface {
	EnrichEventMetadata(context.Context, adapterintake.NormalizedEvent) adapterintake.NormalizedEvent
}

type Deps struct {
	CurrentConfig    func() config.Config
	Logger           *slog.Logger
	Plugins          *plugincatalog.Catalog
	ReplyTargets     *outbound.ReplyTargetCache
	OutboundSender   OutboundActionSender
	OutboundLimiter  outbound.MessageLimiter
	Renderer         *renderservice.Service
	Menu             *menuext.Service
	Bridge           *bridge.Bridge
	Lifecycle        *pluginservice.Controller
	MetadataEnricher MetadataEnricher
	WhitelistRepo    permission.WhitelistRepository
	WhitelistState   permission.WhitelistStateRepository
	BlacklistRepo    permission.BlacklistRepository
}

type Service struct {
	currentConfig    func() config.Config
	logger           *slog.Logger
	plugins          *plugincatalog.Catalog
	replyTargets     *outbound.ReplyTargetCache
	outboundSender   OutboundActionSender
	outboundLimiter  outbound.MessageLimiter
	renderer         *renderservice.Service
	menu             *menuext.Service
	bridge           *bridge.Bridge
	lifecycle        *pluginservice.Controller
	metadataEnricher MetadataEnricher
	policy           *chatpolicy.Service
	whitelistRepo    permission.WhitelistRepository
	whitelistState   permission.WhitelistStateRepository
	blacklistRepo    permission.BlacklistRepository
}

func New(deps Deps) *Service {
	currentConfig := deps.CurrentConfig
	if currentConfig == nil {
		currentConfig = func() config.Config { return config.Config{} }
	}
	service := &Service{
		currentConfig:    currentConfig,
		logger:           deps.Logger,
		plugins:          deps.Plugins,
		replyTargets:     deps.ReplyTargets,
		outboundSender:   deps.OutboundSender,
		outboundLimiter:  deps.OutboundLimiter,
		renderer:         deps.Renderer,
		menu:             deps.Menu,
		bridge:           deps.Bridge,
		lifecycle:        deps.Lifecycle,
		metadataEnricher: deps.MetadataEnricher,
		whitelistRepo:    deps.WhitelistRepo,
		whitelistState:   deps.WhitelistState,
		blacklistRepo:    deps.BlacklistRepo,
	}
	service.policy = chatpolicy.New(chatpolicy.Deps{
		CurrentConfig:   currentConfig,
		Plugins:         deps.Plugins,
		Menu:            deps.Menu,
		Bridge:          deps.Bridge,
		OutboundSender:  deps.OutboundSender,
		OutboundLimiter: deps.OutboundLimiter,
		Logger:          deps.Logger,
		WhitelistRepo:   deps.WhitelistRepo,
		WhitelistState:  deps.WhitelistState,
		BlacklistRepo:   deps.BlacklistRepo,
	})
	return service
}

func (s *Service) UpdateConfig(cfg config.Config) {
	if s == nil {
		return
	}
	if s.policy != nil {
		s.policy.UpdateConfig(cfg)
	}
}

func (s *Service) ApplyChatPolicy(ctx context.Context, event adapterintake.NormalizedEvent) (adapterintake.NormalizedEvent, bool) {
	if s == nil || s.policy == nil {
		return event, true
	}
	s.policy.SetOutboundLimiter(s.outboundLimiter)
	return s.policy.Apply(ctx, event)
}

func (s *Service) EnrichCommandEvent(event adapterintake.NormalizedEvent) adapterintake.NormalizedEvent {
	if s == nil || s.policy == nil {
		return event
	}
	return s.policy.EnrichCommandEvent(event)
}

func (s *Service) CommandInfoForEvent(event adapterintake.NormalizedEvent) *permission.CommandInfo {
	if s == nil || s.policy == nil {
		return nil
	}
	return s.policy.CommandInfoForEvent(event)
}

func (s *Service) SetBridge(eventBridge *bridge.Bridge) {
	if s == nil {
		return
	}
	s.bridge = eventBridge
	if s.policy != nil {
		s.policy.SetBridge(eventBridge)
	}
}

func (s *Service) SetMetadataEnricher(enricher MetadataEnricher) {
	if s != nil {
		s.metadataEnricher = enricher
	}
}

func (s *Service) SetOutboundLimiter(limiter outbound.MessageLimiter) {
	if s == nil {
		return
	}
	s.outboundLimiter = limiter
	if s.policy != nil {
		s.policy.SetOutboundLimiter(limiter)
	}
}

func (s *Service) Policy() *chatpolicy.Service {
	if s == nil {
		return nil
	}
	return s.policy
}
