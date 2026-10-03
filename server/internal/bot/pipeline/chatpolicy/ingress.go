package chatpolicy

import (
	"context"
	"log/slog"
	"maps"
	"sync"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/conversation"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/bot/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

type MetadataEnricher interface {
	EnrichEventMetadata(context.Context, chatevent.NormalizedEvent) chatevent.NormalizedEvent
}

type Lifecycle interface {
	SyncBotIdentities(context.Context)
	HandleAdapterReady(context.Context)
}

type EventBridge interface {
	RejectionLogger
	HandleAdapterEvent(context.Context, chatevent.NormalizedEvent) chatevent.DeliveryOutcome
	QueueAdapterEvent(context.Context, chatevent.NormalizedEvent) (chatevent.DeliveryOutcome, func())
}

type IngressDeps struct {
	MessageReceived  func(chatevent.NormalizedEvent)
	CurrentConfig    func() config.Config
	Logger           *slog.Logger
	Plugins          PluginCatalog
	ReplyTargets     *outbound.ReplyTargetCache
	OutboundSender   OutboundSender
	OutboundLimiter  outbound.MessageAdmitter
	Menu             *menuext.Service
	Bridge           EventBridge
	Lifecycle        Lifecycle
	MetadataEnricher MetadataEnricher
	WhitelistRepo    permission.EntryRepository
	WhitelistState   permission.WhitelistStateRepository
	BlacklistRepo    permission.EntryRepository
	Conversations    *conversation.Registry
}

type Ingress struct {
	messageReceived  func(chatevent.NormalizedEvent)
	admissionMu      sync.Mutex
	stopping         bool
	active           map[*context.CancelFunc]struct{}
	drained          chan struct{}
	replyTargets     *outbound.ReplyTargetCache
	menu             *menuext.Service
	bridge           EventBridge
	lifecycle        Lifecycle
	metadataEnricher MetadataEnricher
	policy           *Service
	conversations    *conversation.Registry
}

func NewIngress(deps IngressDeps) *Ingress {
	if deps.MessageReceived == nil {
		deps.MessageReceived = func(chatevent.NormalizedEvent) {}
	}
	currentConfig := deps.CurrentConfig
	if currentConfig == nil {
		currentConfig = func() config.Config { return config.Config{} }
	}
	service := &Ingress{
		messageReceived:  deps.MessageReceived,
		active:           make(map[*context.CancelFunc]struct{}),
		drained:          make(chan struct{}),
		replyTargets:     deps.ReplyTargets,
		menu:             deps.Menu,
		bridge:           deps.Bridge,
		lifecycle:        deps.Lifecycle,
		metadataEnricher: deps.MetadataEnricher,
		conversations:    deps.Conversations,
	}
	policyDeps := Deps{
		CurrentConfig:   currentConfig,
		Plugins:         deps.Plugins,
		OutboundSender:  deps.OutboundSender,
		OutboundLimiter: deps.OutboundLimiter,
		Logger:          deps.Logger,
		WhitelistRepo:   deps.WhitelistRepo,
		WhitelistState:  deps.WhitelistState,
		BlacklistRepo:   deps.BlacklistRepo,
	}
	// Keep absent collaborators as nil interfaces.
	if deps.Menu != nil {
		policyDeps.Menu = deps.Menu
	}
	if deps.Bridge != nil {
		policyDeps.Bridge = deps.Bridge
	}
	service.policy = New(policyDeps)
	return service
}

func (s *Ingress) UpdateConfig(cfg config.Config) {
	if s.policy != nil {
		s.policy.UpdateConfig(cfg)
	}
	if s.menu != nil {
		s.menu.UpdateConfig(cfg)
	}
}

func (s *Ingress) ApplyChatPolicy(ctx context.Context, event chatevent.NormalizedEvent) (chatevent.NormalizedEvent, bool) {
	if s.policy == nil {
		return event, true
	}
	return s.policy.Apply(ctx, event)
}

func (s *Ingress) Policy() *Service {
	return s.policy
}

func (s *Ingress) HandleAdapterEvent(ctx context.Context, event chatevent.NormalizedEvent) {
	s.messageReceived(event)
	s.admissionMu.Lock()
	if s.stopping {
		s.admissionMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	s.active[&cancel] = struct{}{}
	s.admissionMu.Unlock()
	defer func() {
		cancel()
		s.admissionMu.Lock()
		delete(s.active, &cancel)
		if s.stopping && len(s.active) == 0 {
			close(s.drained)
		}
		s.admissionMu.Unlock()
	}()
	event = s.enrichEventMetadata(ctx, event)
	if ctx.Err() != nil {
		return
	}
	if s.replyTargets != nil {
		s.replyTargets.Record(event)
	}
	if s.conversations != nil && s.conversations.HasWaiting(chatevent.FromAdapter(event)) {
		if !s.policy.AllowSessionInput(ctx, event) {
			return
		}
		if s.lifecycle != nil {
			s.lifecycle.SyncBotIdentities(ctx)
		}
		sessionEvent := event
		sessionEvent.PayloadFields = maps.Clone(event.PayloadFields)
		delete(sessionEvent.PayloadFields, "command")
		delete(sessionEvent.PayloadFields, "args")
		var report func()
		consumed := s.conversations.TryRoute(chatevent.FromAdapter(sessionEvent), func(owner conversation.Owner, ref chatevent.SessionRef) bool {
			if s.bridge == nil {
				return false
			}
			var outcome chatevent.DeliveryOutcome
			outcome, report = s.bridge.QueueAdapterEvent(conversation.WithDelivery(ctx, owner, ref), sessionEvent)
			return outcome == chatevent.DeliveryOutcomeDelivered
		})
		if report != nil {
			report()
		}
		if consumed {
			return
		}
	}

	enriched, allowed := s.ApplyChatPolicy(ctx, event)
	if !allowed {
		return
	}

	if s.menu != nil && s.menu.Handle(ctx, enriched) {
		return
	}

	if s.lifecycle != nil {
		s.lifecycle.SyncBotIdentities(ctx)
	}

	if s.bridge != nil {
		s.bridge.HandleAdapterEvent(ctx, enriched)
	}
}

// StopAdmission leaves accepted handlers and outbound transports alive.
func (s *Ingress) StopAdmission() {
	s.admissionMu.Lock()
	defer s.admissionMu.Unlock()
	if s.stopping {
		return
	}
	s.stopping = true
	if len(s.active) == 0 {
		close(s.drained)
	}
}

// Drain shares the dispatcher budget; expired handlers are cancelled before
// transport shutdown joins them and releases their dependencies.
func (s *Ingress) Drain(ctx context.Context) error {
	s.StopAdmission()
	select {
	case <-s.drained:
		return nil
	default:
	}
	select {
	case <-s.drained:
		return nil
	case <-ctx.Done():
		s.admissionMu.Lock()
		for cancel := range s.active {
			(*cancel)()
		}
		s.admissionMu.Unlock()
		return ctx.Err()
	}
}

func (s *Ingress) enrichEventMetadata(ctx context.Context, event chatevent.NormalizedEvent) chatevent.NormalizedEvent {
	if s.metadataEnricher == nil {
		return event
	}
	return s.metadataEnricher.EnrichEventMetadata(ctx, event)
}

func (s *Ingress) HandleAdapterReady(ctx context.Context) {
	s.admissionMu.Lock()
	stopping := s.stopping
	s.admissionMu.Unlock()
	if stopping {
		return
	}
	if s.lifecycle == nil {
		return
	}

	s.lifecycle.HandleAdapterReady(ctx)
}
