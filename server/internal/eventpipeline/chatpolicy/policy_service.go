package chatpolicy

import (
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	menuext "github.com/RayleaBot/RayleaBot/server/internal/builtinmenu"
	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/command"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type PluginCatalog interface {
	List() []plugins.Snapshot
}

type MenuMatcher interface {
	Match(chatevent.NormalizedEvent) menuext.Request
}

type RejectionLogger interface {
	LogCommandPolicyRejected(chatevent.NormalizedEvent, bridge.CommandPolicyRejection)
}

type OutboundSender interface {
	SendMessage(context.Context, chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error)
	SendReply(context.Context, chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error)
}

type Deps struct {
	CurrentConfig   func() config.Config
	Plugins         PluginCatalog
	Menu            MenuMatcher
	Bridge          RejectionLogger
	OutboundSender  OutboundSender
	OutboundLimiter outbound.MessageLimiter
	Logger          *slog.Logger
	WhitelistRepo   permission.EntryRepository
	WhitelistState  permission.WhitelistStateRepository
	BlacklistRepo   permission.EntryRepository
}

// policyEngine bundles the config-derived policy collaborators into one
// immutable snapshot so hot reload can swap them atomically while event
// goroutines keep reading a consistent set.
type policyEngine struct {
	parser   *command.Parser
	checker  *permission.Checker
	cooldown *permission.CooldownTracker
	snapshot ConfigSnapshot
}

type Service struct {
	currentConfig   func() config.Config
	plugins         PluginCatalog
	menu            MenuMatcher
	bridge          RejectionLogger
	outboundSender  OutboundSender
	outboundLimiter outbound.MessageLimiter
	logger          *slog.Logger
	whitelistRepo   permission.EntryRepository
	whitelistState  permission.WhitelistStateRepository
	blacklistRepo   permission.EntryRepository
	engine          atomic.Pointer[policyEngine]
}

func New(deps Deps) *Service {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
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
	settings := ResolveConfig(cfg)
	previous := s.engine.Load()

	// Preserve in-flight cooldown windows when the rate-limit fields did not
	// change, so unrelated config updates do not reset every user/group
	// cooldown counter.
	var cooldown *permission.CooldownTracker
	if previous != nil &&
		previous.snapshot.UserCommandRateLimit == settings.UserCommandRateLimit &&
		previous.snapshot.GroupCommandRateLimit == settings.GroupCommandRateLimit {
		cooldown = previous.cooldown
	} else {
		userLimit := parseCooldownRateLimitWithFallback(settings.UserCommandRateLimit, config.DefaultUserCommandRateLimit)
		groupLimit := parseCooldownRateLimitWithFallback(settings.GroupCommandRateLimit, config.DefaultGroupCommandRateLimit)
		cooldown = permission.NewCooldownTracker(userLimit, groupLimit)
	}

	checker := permission.NewChecker(permission.CheckerConfig{
		SuperAdmins:  settings.SuperAdmins,
		DefaultLevel: settings.DefaultLevel,
	}, s.whitelistRepo, s.whitelistState, s.blacklistRepo, cooldown)

	s.engine.Store(&policyEngine{
		parser:   newCommandParser(cfg),
		checker:  checker,
		cooldown: cooldown,
		snapshot: settings,
	})
}

func (s *Service) currentEngine() *policyEngine {
	return s.engine.Load()
}

func (s *Service) CommandParser() *command.Parser {
	engine := s.currentEngine()
	if engine == nil {
		return nil
	}
	return engine.parser
}

func (s *Service) PermissionChecker() *permission.Checker {
	engine := s.currentEngine()
	if engine == nil {
		return nil
	}
	return engine.checker
}

func (s *Service) SetOutboundLimiter(limiter outbound.MessageLimiter) {
	s.outboundLimiter = limiter
}

func (s *Service) config() config.Config {
	if s.currentConfig == nil {
		return config.Config{}
	}
	return s.currentConfig()
}

func (s *Service) Apply(ctx context.Context, event chatevent.NormalizedEvent) (chatevent.NormalizedEvent, bool) {
	enriched := s.EnrichCommandEvent(event)
	checker := s.PermissionChecker()
	if checker == nil || !shouldEvaluateChatPolicy(enriched) {
		return enriched, true
	}
	commandContext := s.commandPolicyContextForEvent(enriched)

	var permissionInfo *permission.CommandInfo
	if commandContext != nil {
		permissionInfo = commandContext.PermissionInfo
	}

	verdict := checker.Check(
		ctx,
		enriched.IdentityScope(),
		strings.TrimSpace(enriched.SenderID),
		strings.TrimSpace(enriched.ActorRole),
		commandGroupID(enriched),
		permissionInfo,
	)
	if verdict.Allowed {
		return enriched, true
	}
	if verdict.Err != nil {
		s.logger.ErrorContext(ctx, "权限数据读取失败，本次事件未执行", "error_code", verdict.ErrorCode, "err", verdict.Err,
			"event_id", enriched.EventID, "source_adapter", enriched.SourceAdapter)
	}

	if commandContext != nil {
		s.logCommandPolicyRejection(enriched, verdict, commandContext)
	}
	if (verdict.ErrorCode == errorcodes.PlatformUserRateLimited || verdict.ErrorCode == errorcodes.PlatformRateLimited) && cooldownReplyEnabled(s.config()) {
		s.sendCooldownReply(ctx, enriched)
	}
	return enriched, false
}

func shouldEvaluateChatPolicy(event chatevent.NormalizedEvent) bool {
	switch chatevent.EventFamily(event.Kind) {
	case chatevent.FamilyMessageText, chatevent.FamilyMessage, chatevent.FamilyNotice:
		return true
	default:
		return false
	}
}

func commandGroupID(event chatevent.NormalizedEvent) string {
	if event.ConversationType != "group" {
		return ""
	}
	return strings.TrimSpace(event.ConversationID)
}

type ConfigSnapshot struct {
	SuperAdmins           []string
	DefaultLevel          string
	UserCommandRateLimit  string
	GroupCommandRateLimit string
	CooldownReplyEnabled  bool
}

func parseCooldownRateLimitWithFallback(raw, fallback string) config.RateLimit {
	if limit, err := config.ParseRateLimit(strings.TrimSpace(raw)); err == nil {
		return limit
	}
	return parseCooldownRateLimit(fallback)
}

func parseCooldownRateLimit(raw string) config.RateLimit {
	limit, err := config.ParseRateLimit(raw)
	if err == nil {
		return limit
	}
	return config.RateLimit{Count: 1, Window: time.Minute}
}

func commandPermissionDefaultLevel(cfg config.Config) string {
	defaultLevel := strings.TrimSpace(ResolveConfig(cfg).DefaultLevel)
	switch defaultLevel {
	case "super_admin", "group_admin", "everyone":
		return defaultLevel
	default:
		return "everyone"
	}
}

func cooldownReplyEnabled(cfg config.Config) bool {
	return ResolveConfig(cfg).CooldownReplyEnabled
}

func ResolveConfig(cfg config.Config) ConfigSnapshot {
	settings := ConfigSnapshot{
		SuperAdmins:           append([]string(nil), cfg.Admin.SuperAdmins...),
		DefaultLevel:          strings.TrimSpace(cfg.Permission.DefaultLevel),
		UserCommandRateLimit:  strings.TrimSpace(cfg.User.CommandRateLimit),
		GroupCommandRateLimit: strings.TrimSpace(cfg.Group.CommandRateLimit),
		CooldownReplyEnabled:  cfg.User.CooldownReply,
	}

	if settings.UserCommandRateLimit == "" {
		settings.UserCommandRateLimit = config.DefaultUserCommandRateLimit
	}
	if settings.GroupCommandRateLimit == "" {
		settings.GroupCommandRateLimit = config.DefaultGroupCommandRateLimit
	}
	if settings.DefaultLevel == "" {
		settings.DefaultLevel = "everyone"
	}
	return settings
}

func (s *Service) logCommandPolicyRejection(event chatevent.NormalizedEvent, verdict permission.Verdict, commandContext *commandPolicyContext) {
	if s.bridge == nil || commandContext == nil {
		return
	}

	s.bridge.LogCommandPolicyRejected(event, bridge.CommandPolicyRejection{
		CommandName:      commandContext.CommandName,
		PluginID:         commandContext.PrimaryPluginID,
		MatchedPluginIDs: commandContext.MatchedPluginIDs,
		ErrorCode:        verdict.ErrorCode,
		Reason:           verdict.Reason,
		ReasonSummary:    commandPolicyReasonSummary(verdict),
		PolicyStage:      commandPolicyStage(verdict.ErrorCode),
	})
}

func commandPolicyStage(errorCode string) string {
	switch strings.TrimSpace(errorCode) {
	case errorcodes.PermissionNotWhitelisted:
		return "whitelist"
	case errorcodes.PermissionBlacklisted:
		return "blacklist"
	case errorcodes.PermissionDenied, errorcodes.PermissionUnavailable:
		return "permission"
	case errorcodes.PlatformUserRateLimited, errorcodes.PlatformRateLimited:
		return "cooldown"
	default:
		return ""
	}
}

func commandPolicyReasonSummary(verdict permission.Verdict) string {
	switch strings.TrimSpace(verdict.ErrorCode) {
	case errorcodes.PermissionNotWhitelisted:
		return "发送者不在白名单中"
	case errorcodes.PermissionBlacklisted:
		if verdict.Scope == permission.ScopeGroup {
			return "群在黑名单中"
		}
		return "用户在黑名单中"
	case errorcodes.PermissionDenied:
		return "权限等级不足"
	case errorcodes.PlatformUserRateLimited:
		return "用户命令触发频率限制"
	case errorcodes.PlatformRateLimited:
		return "群命令触发频率限制"
	default:
		return strings.TrimSpace(verdict.Reason)
	}
}
