package chatpolicy

import (
	"context"
	"log/slog"
	"strings"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/command"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type PluginCatalog interface {
	List() []plugins.Snapshot
}

type MenuMatcher interface {
	Match(adapterintake.NormalizedEvent) menuext.Request
}

type RejectionLogger interface {
	LogCommandPolicyRejected(adapterintake.NormalizedEvent, bridge.CommandPolicyRejection)
}

type OutboundSender interface {
	SendMessage(context.Context, adapteroutbound.OutboundMessageSend) (adapteroutbound.SendMessageResult, error)
	SendReply(context.Context, adapteroutbound.OutboundMessageReply) (adapteroutbound.SendMessageResult, error)
}

type Deps struct {
	CurrentConfig   func() config.Config
	Plugins         PluginCatalog
	Menu            MenuMatcher
	Bridge          RejectionLogger
	OutboundSender  OutboundSender
	OutboundLimiter outbound.MessageLimiter
	Logger          *slog.Logger
	WhitelistRepo   permission.WhitelistRepository
	WhitelistState  permission.WhitelistStateRepository
	BlacklistRepo   permission.BlacklistRepository
}

type Service struct {
	currentConfig     func() config.Config
	plugins           PluginCatalog
	menu              MenuMatcher
	bridge            RejectionLogger
	outboundSender    OutboundSender
	outboundLimiter   outbound.MessageLimiter
	logger            *slog.Logger
	whitelistRepo     permission.WhitelistRepository
	whitelistState    permission.WhitelistStateRepository
	blacklistRepo     permission.BlacklistRepository
	commandParser     *command.Parser
	permissionChecker *permission.Checker
}

func New(deps Deps) *Service {
	service := &Service{
		currentConfig:   deps.CurrentConfig,
		plugins:         deps.Plugins,
		menu:            deps.Menu,
		bridge:          deps.Bridge,
		outboundSender:  deps.OutboundSender,
		outboundLimiter: deps.OutboundLimiter,
		logger:          deps.Logger,
		whitelistRepo:   deps.WhitelistRepo,
		whitelistState:  deps.WhitelistState,
		blacklistRepo:   deps.BlacklistRepo,
	}
	service.UpdateConfig(service.config())
	return service
}

func (s *Service) UpdateConfig(cfg config.Config) {
	if s == nil {
		return
	}
	s.commandParser = newCommandParser(cfg)
	s.permissionChecker = newPermissionChecker(cfg, s.whitelistRepo, s.whitelistState, s.blacklistRepo)
}

func (s *Service) CommandParser() *command.Parser {
	if s == nil {
		return nil
	}
	return s.commandParser
}

func (s *Service) PermissionChecker() *permission.Checker {
	if s == nil {
		return nil
	}
	return s.permissionChecker
}

func (s *Service) SetBridge(logger RejectionLogger) {
	if s == nil {
		return
	}
	s.bridge = logger
}

func (s *Service) SetOutboundLimiter(limiter outbound.MessageLimiter) {
	if s == nil {
		return
	}
	s.outboundLimiter = limiter
}

func (s *Service) config() config.Config {
	if s == nil || s.currentConfig == nil {
		return config.Config{}
	}
	return s.currentConfig()
}

func (s *Service) Apply(ctx context.Context, event adapterintake.NormalizedEvent) (adapterintake.NormalizedEvent, bool) {
	enriched := s.EnrichCommandEvent(event)
	if s == nil || s.permissionChecker == nil || !shouldEvaluateChatPolicy(enriched) {
		return enriched, true
	}
	commandContext := s.commandPolicyContextForEvent(enriched)

	var permissionInfo *permission.CommandInfo
	if commandContext != nil {
		permissionInfo = commandContext.PermissionInfo
	}

	verdict := s.permissionChecker.Check(
		ctx,
		strings.TrimSpace(enriched.SenderID),
		strings.TrimSpace(enriched.ActorRole),
		commandGroupID(enriched),
		permissionInfo,
	)
	if verdict.Allowed {
		return enriched, true
	}

	if commandContext != nil {
		s.logCommandPolicyRejection(enriched, verdict, commandContext)
	}
	if (verdict.ErrorCode == "platform.user_rate_limited" || verdict.ErrorCode == "platform.rate_limited") && cooldownReplyEnabled(s.config()) {
		s.sendCooldownReply(ctx, enriched)
	}
	return enriched, false
}

func shouldEvaluateChatPolicy(event adapterintake.NormalizedEvent) bool {
	switch event.Kind {
	case adapterintake.EventKindMessageText, adapterintake.EventKindMessage, adapterintake.EventKindNotice:
		return true
	default:
		return false
	}
}

func commandGroupID(event adapterintake.NormalizedEvent) string {
	if event.ConversationType != "group" {
		return ""
	}
	return strings.TrimSpace(event.ConversationID)
}
