package chatpolicy

import (
	"context"
	"log/slog"

	menuext "github.com/RayleaBot/RayleaBot/server/internal/builtinmenu"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
)

type MetadataEnricher interface {
	EnrichEventMetadata(context.Context, onebot11.NormalizedEvent) onebot11.NormalizedEvent
}

type IngressDeps struct {
	CurrentConfig    func() config.Config
	Logger           *slog.Logger
	Plugins          *plugincatalog.Catalog
	ReplyTargets     *outbound.ReplyTargetCache
	OutboundSender   OutboundSender
	OutboundLimiter  outbound.MessageLimiter
	Menu             *menuext.Service
	Bridge           *bridge.Bridge
	Lifecycle        *pluginservice.Controller
	MetadataEnricher MetadataEnricher
	WhitelistRepo    permission.WhitelistRepository
	WhitelistState   permission.WhitelistStateRepository
	BlacklistRepo    permission.BlacklistRepository
}

type Ingress struct {
	replyTargets     *outbound.ReplyTargetCache
	outboundLimiter  outbound.MessageLimiter
	menu             *menuext.Service
	bridge           *bridge.Bridge
	lifecycle        *pluginservice.Controller
	metadataEnricher MetadataEnricher
	policy           *Service
}

func NewIngress(deps IngressDeps) *Ingress {
	currentConfig := deps.CurrentConfig
	if currentConfig == nil {
		currentConfig = func() config.Config { return config.Config{} }
	}
	service := &Ingress{
		replyTargets:     deps.ReplyTargets,
		outboundLimiter:  deps.OutboundLimiter,
		menu:             deps.Menu,
		bridge:           deps.Bridge,
		lifecycle:        deps.Lifecycle,
		metadataEnricher: deps.MetadataEnricher,
	}
	service.policy = New(Deps{
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

func (s *Ingress) UpdateConfig(cfg config.Config) {
	if s == nil {
		return
	}
	if s.policy != nil {
		s.policy.UpdateConfig(cfg)
	}
}

func (s *Ingress) ApplyChatPolicy(ctx context.Context, event onebot11.NormalizedEvent) (onebot11.NormalizedEvent, bool) {
	if s == nil || s.policy == nil {
		return event, true
	}
	s.policy.SetOutboundLimiter(s.outboundLimiter)
	return s.policy.Apply(ctx, event)
}

func (s *Ingress) EnrichCommandEvent(event onebot11.NormalizedEvent) onebot11.NormalizedEvent {
	if s == nil || s.policy == nil {
		return event
	}
	return s.policy.EnrichCommandEvent(event)
}

func (s *Ingress) CommandInfoForEvent(event onebot11.NormalizedEvent) *permission.CommandInfo {
	if s == nil || s.policy == nil {
		return nil
	}
	return s.policy.CommandInfoForEvent(event)
}

func (s *Ingress) SetMetadataEnricher(enricher MetadataEnricher) {
	if s != nil {
		s.metadataEnricher = enricher
	}
}

func (s *Ingress) SetOutboundLimiter(limiter outbound.MessageLimiter) {
	if s == nil {
		return
	}
	s.outboundLimiter = limiter
	if s.policy != nil {
		s.policy.SetOutboundLimiter(limiter)
	}
}

func (s *Ingress) Policy() *Service {
	if s == nil {
		return nil
	}
	return s.policy
}

func (s *Ingress) HandleAdapterEvent(ctx context.Context, event onebot11.NormalizedEvent) {
	if s == nil {
		return
	}
	event = s.enrichEventMetadata(ctx, event)
	if s.replyTargets != nil {
		s.replyTargets.Record(event)
	}

	enriched, allowed := s.ApplyChatPolicy(ctx, event)
	if !allowed {
		return
	}

	if s.menu != nil && s.menu.Handle(ctx, enriched) {
		return
	}

	if s.lifecycle != nil {
		s.lifecycle.HandleAdapterBotID(ctx, event.BotID)
	}

	if s.bridge != nil {
		s.bridge.HandleAdapterEvent(ctx, enriched)
	}
}

func (s *Ingress) enrichEventMetadata(ctx context.Context, event onebot11.NormalizedEvent) onebot11.NormalizedEvent {
	if s == nil || s.metadataEnricher == nil {
		return event
	}
	return s.metadataEnricher.EnrichEventMetadata(ctx, event)
}

func (s *Ingress) HandleAdapterReady(ctx context.Context) {
	if s == nil || s.lifecycle == nil {
		return
	}

	s.lifecycle.HandleAdapterReady(ctx)
}
