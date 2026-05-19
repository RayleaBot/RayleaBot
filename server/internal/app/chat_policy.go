package app

import (
	"context"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/command"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

const (
	defaultUserCommandRateLimit  = "10/60s"
	defaultGroupCommandRateLimit = "30/60s"
	cooldownReplyText            = "命令触发冷却，请稍后再试。"
)

type outboundActionSender interface {
	SendMessage(context.Context, adapter.OutboundMessageSend) (adapter.SendMessageResult, error)
	SendReply(context.Context, adapter.OutboundMessageReply) (adapter.SendMessageResult, error)
}

type chatPolicyConfigSnapshot struct {
	SuperAdmins           []string
	DefaultLevel          string
	UserCommandRateLimit  string
	GroupCommandRateLimit string
	CooldownReplyEnabled  bool
}

func newPermissionChecker(cfg config.Config, whitelistRepo permission.WhitelistRepository, whitelistState permission.WhitelistStateRepository, blacklistRepo permission.BlacklistRepository) *permission.Checker {
	settings := resolveChatPolicyConfig(cfg)
	userLimit := parseCooldownRateLimitWithFallback(settings.UserCommandRateLimit, defaultUserCommandRateLimit)
	groupLimit := parseCooldownRateLimitWithFallback(settings.GroupCommandRateLimit, defaultGroupCommandRateLimit)

	return permission.NewChecker(permission.CheckerConfig{
		SuperAdmins:  append([]string(nil), settings.SuperAdmins...),
		DefaultLevel: settings.DefaultLevel,
	}, whitelistRepo, whitelistState, blacklistRepo, permission.NewCooldownTracker(userLimit, groupLimit))
}

func parseCooldownRateLimitWithFallback(raw, fallback string) permission.RateLimit {
	if limit, err := permission.ParseRateLimit(strings.TrimSpace(raw)); err == nil {
		return limit
	}
	return parseCooldownRateLimit(fallback)
}

func parseCooldownRateLimit(raw string) permission.RateLimit {
	limit, err := permission.ParseRateLimit(raw)
	if err == nil {
		return limit
	}
	return permission.RateLimit{Count: 1, Window: time.Minute}
}

func commandPermissionDefaultLevel(cfg config.Config) string {
	defaultLevel := strings.TrimSpace(resolveChatPolicyConfig(cfg).DefaultLevel)
	switch defaultLevel {
	case "super_admin", "group_admin", "everyone":
		return defaultLevel
	default:
		return "everyone"
	}
}

func cooldownReplyEnabled(cfg config.Config) bool {
	return resolveChatPolicyConfig(cfg).CooldownReplyEnabled
}

func resolveChatPolicyConfig(cfg config.Config) chatPolicyConfigSnapshot {
	settings := chatPolicyConfigSnapshot{
		SuperAdmins:           append([]string(nil), cfg.Admin.SuperAdmins...),
		DefaultLevel:          strings.TrimSpace(cfg.Permission.DefaultLevel),
		UserCommandRateLimit:  strings.TrimSpace(cfg.User.CommandRateLimit),
		GroupCommandRateLimit: strings.TrimSpace(cfg.Group.CommandRateLimit),
		CooldownReplyEnabled:  cfg.User.CooldownReply,
	}

	if len(settings.SuperAdmins) == 0 && len(cfg.Auth.SuperAdmins) > 0 {
		settings.SuperAdmins = append([]string(nil), cfg.Auth.SuperAdmins...)
	}
	if settings.DefaultLevel == "" {
		settings.DefaultLevel = strings.TrimSpace(cfg.Auth.DefaultLevel)
	}
	if settings.UserCommandRateLimit == "" && cfg.Cooldown != nil {
		settings.UserCommandRateLimit = strings.TrimSpace(cfg.Cooldown.UserCommandRateLimit)
	}
	if settings.GroupCommandRateLimit == "" && cfg.Cooldown != nil {
		settings.GroupCommandRateLimit = strings.TrimSpace(cfg.Cooldown.GroupCommandRateLimit)
	}
	if canonicalCooldownReplyConfigured(cfg) {
		settings.CooldownReplyEnabled = cfg.User.CooldownReply
	} else if cfg.Cooldown != nil && cfg.Cooldown.CooldownReply {
		settings.CooldownReplyEnabled = true
	}
	if settings.UserCommandRateLimit == "" {
		settings.UserCommandRateLimit = defaultUserCommandRateLimit
	}
	if settings.GroupCommandRateLimit == "" {
		settings.GroupCommandRateLimit = defaultGroupCommandRateLimit
	}
	if !settings.CooldownReplyEnabled &&
		settings.UserCommandRateLimit == defaultUserCommandRateLimit &&
		settings.GroupCommandRateLimit == defaultGroupCommandRateLimit &&
		cfg.Cooldown == nil &&
		len(cfg.Admin.SuperAdmins) == 0 &&
		len(cfg.Permission.AutoGrantCapabilities) == 0 &&
		len(cfg.Auth.SuperAdmins) == 0 &&
		strings.TrimSpace(cfg.Permission.DefaultLevel) == "" &&
		strings.TrimSpace(cfg.Auth.DefaultLevel) == "" &&
		strings.TrimSpace(cfg.User.CommandRateLimit) == "" &&
		strings.TrimSpace(cfg.Group.CommandRateLimit) == "" {
		settings.CooldownReplyEnabled = true
	}
	if settings.DefaultLevel == "" {
		settings.DefaultLevel = "everyone"
	}
	return settings
}

func canonicalCooldownReplyConfigured(cfg config.Config) bool {
	return strings.TrimSpace(cfg.User.CommandRateLimit) != "" || cfg.User.CooldownReply || cfg.Cooldown == nil
}

type eventIngressService struct {
	state             *appRuntimeState
	plugins           *plugins.Catalog
	replyTargets      *replyTargetCache
	outboundSender    outboundActionSender
	outboundLimiter   outbound.MessageLimiter
	renderer          *render.Service
	menu              *menuext.Service
	bridge            *bridge.Bridge
	lifecycle         *pluginLifecycleController
	metadataEnricher  eventMetadataEnricher
	commandParser     *command.Parser
	permissionChecker *permission.Checker
	whitelistRepo     permission.WhitelistRepository
	whitelistState    permission.WhitelistStateRepository
	blacklistRepo     permission.BlacklistRepository
}

type commandPolicyContext struct {
	CommandName      string
	PermissionInfo   *permission.CommandInfo
	MatchedPluginIDs []string
	PrimaryPluginID  string
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

func (s *eventIngressService) applyChatPolicy(ctx context.Context, event adapter.NormalizedEvent) (adapter.NormalizedEvent, bool) {
	enriched := s.enrichCommandEvent(event)
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
	if (verdict.ErrorCode == "platform.user_rate_limited" || verdict.ErrorCode == "platform.rate_limited") && cooldownReplyEnabled(s.state.Config) {
		s.sendCooldownReply(ctx, enriched)
	}
	return enriched, false
}

func shouldEvaluateChatPolicy(event adapter.NormalizedEvent) bool {
	switch event.Kind {
	case adapter.EventKindMessageText, adapter.EventKindMessage, adapter.EventKindNotice:
		return true
	default:
		return false
	}
}

func commandGroupID(event adapter.NormalizedEvent) string {
	if event.ConversationType != "group" {
		return ""
	}
	return strings.TrimSpace(event.ConversationID)
}

func (s *eventIngressService) commandInfoForEvent(event adapter.NormalizedEvent) *permission.CommandInfo {
	commandName := commandNameFromEvent(event)
	if commandName == "" {
		return nil
	}

	commandContext := s.commandPolicyContextForEvent(event)
	if commandContext == nil {
		return nil
	}
	return commandContext.PermissionInfo
}

func (s *eventIngressService) commandPolicyContextForEvent(event adapter.NormalizedEvent) *commandPolicyContext {
	commandName := commandNameFromEvent(event)
	if commandName == "" {
		return nil
	}

	requiredLevel := "everyone"
	context := &commandPolicyContext{
		CommandName:    commandName,
		PermissionInfo: &permission.CommandInfo{Permission: requiredLevel},
	}
	currentConfig := config.Config{}
	if s != nil && s.state != nil {
		currentConfig = s.state.Config
	}
	if s != nil && s.plugins != nil {
		for _, snapshot := range s.plugins.List() {
			if !pluginParticipatesInCommandPolicy(snapshot) {
				continue
			}
			for _, command := range snapshot.Commands {
				if !commandMatches(command, commandName) {
					continue
				}
				context.MatchedPluginIDs = append(context.MatchedPluginIDs, snapshot.PluginID)
				level := effectiveCommandPermissionLevel(command.Permission, currentConfig)
				if commandPermissionRank(level) > commandPermissionRank(requiredLevel) {
					requiredLevel = level
				}
				break
			}
		}
	}
	if s != nil && s.menu != nil && s.menu.Match(event).Matched {
		context.MatchedPluginIDs = nil
		requiredLevel = "everyone"
	}

	context.PermissionInfo.Permission = requiredLevel
	if len(context.MatchedPluginIDs) == 1 {
		context.PrimaryPluginID = context.MatchedPluginIDs[0]
	}
	return context
}

func commandNameFromEvent(event adapter.NormalizedEvent) string {
	if event.PayloadFields == nil {
		return ""
	}
	value, ok := event.PayloadFields["command"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func pluginParticipatesInCommandPolicy(snapshot plugins.Snapshot) bool {
	return snapshot.Valid &&
		snapshot.RegistrationState == "installed" &&
		snapshot.DesiredState == "enabled"
}

func commandMatches(command plugins.Command, commandName string) bool {
	if strings.TrimSpace(command.Name) == commandName {
		return true
	}
	for _, alias := range command.Aliases {
		if strings.TrimSpace(alias) == commandName {
			return true
		}
	}
	return false
}

func effectiveCommandPermissionLevel(permissionLevel string, cfg config.Config) string {
	switch strings.TrimSpace(permissionLevel) {
	case "super_admin", "group_admin", "everyone":
		return strings.TrimSpace(permissionLevel)
	case "":
		return commandPermissionDefaultLevel(cfg)
	default:
		return "everyone"
	}
}

func commandPermissionRank(level string) int {
	switch level {
	case "super_admin":
		return 3
	case "group_admin":
		return 2
	case "everyone":
		return 1
	default:
		return 1
	}
}

func (s *eventIngressService) logCommandPolicyRejection(event adapter.NormalizedEvent, verdict permission.Verdict, commandContext *commandPolicyContext) {
	if s == nil || s.bridge == nil || commandContext == nil {
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
	case "permission.not_whitelisted":
		return "whitelist"
	case "permission.blacklisted":
		return "blacklist"
	case "permission.denied":
		return "permission"
	case "platform.user_rate_limited", "platform.rate_limited":
		return "cooldown"
	default:
		return ""
	}
}

func commandPolicyReasonSummary(verdict permission.Verdict) string {
	switch strings.TrimSpace(verdict.ErrorCode) {
	case "permission.not_whitelisted":
		return "sender is not whitelisted"
	case "permission.blacklisted":
		if strings.TrimSpace(verdict.Reason) == "group is blacklisted" {
			return "group is blacklisted"
		}
		return "user is blacklisted"
	case "permission.denied":
		return "insufficient permission level"
	case "platform.user_rate_limited":
		return "user command rate limited"
	case "platform.rate_limited":
		return "group command rate limited"
	default:
		return strings.TrimSpace(verdict.Reason)
	}
}

func (s *eventIngressService) sendCooldownReply(ctx context.Context, event adapter.NormalizedEvent) {
	if s == nil || s.outboundSender == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var (
		attempt outbound.SendAttempt
		result  outbound.SendResult
		err     error
	)

	switch strings.TrimSpace(event.ConversationType) {
	case "group":
		if messageID := strings.TrimSpace(event.MessageID); messageID != "" {
			segments := []adapter.OutboundMessageSegment{{
				Type: "text",
				Data: map[string]any{"text": cooldownReplyText},
			}}
			attempt = outbound.SendAttempt{
				ActionKind: "message.reply",
				TargetType: "group",
				TargetID:   strings.TrimSpace(event.ConversationID),
				Segments:   segments,
			}
			result = outbound.SendResult{
				DeliveryKind: "message.reply",
				TargetType:   "group",
				TargetID:     strings.TrimSpace(event.ConversationID),
			}
			if limitErr := s.waitOutboundLimit(ctx, outbound.MessageLimitRequest{
				TargetType: result.TargetType,
				TargetID:   result.TargetID,
			}); limitErr != nil {
				err = limitErr
				break
			}
			sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			sendResult, sendErr := s.outboundSender.SendReply(sendCtx, adapter.OutboundMessageReply{
				TargetType:       "group",
				TargetID:         strings.TrimSpace(event.ConversationID),
				ReplyToMessageID: messageID,
				Segments:         segments,
			})
			cancel()
			result.MessageID = sendResult.MessageID
			err = sendErr
			break
		}
		fallthrough
	case "private":
		if targetID := strings.TrimSpace(event.ConversationID); targetID != "" {
			segments := []adapter.OutboundMessageSegment{{
				Type: "text",
				Data: map[string]any{"text": cooldownReplyText},
			}}
			attempt = outbound.SendAttempt{
				ActionKind: "message.send",
				TargetType: strings.TrimSpace(event.ConversationType),
				TargetID:   targetID,
				Segments:   segments,
			}
			result = outbound.SendResult{
				DeliveryKind: "message.send",
				TargetType:   strings.TrimSpace(event.ConversationType),
				TargetID:     targetID,
			}
			if limitErr := s.waitOutboundLimit(ctx, outbound.MessageLimitRequest{
				TargetType: result.TargetType,
				TargetID:   result.TargetID,
			}); limitErr != nil {
				err = limitErr
				break
			}
			sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			sendResult, sendErr := s.outboundSender.SendMessage(sendCtx, adapter.OutboundMessageSend{
				TargetType: strings.TrimSpace(event.ConversationType),
				TargetID:   targetID,
				Segments:   segments,
			})
			cancel()
			result.MessageID = sendResult.MessageID
			err = sendErr
		}
	default:
		return
	}

	if s.state != nil && s.state.Logger != nil && strings.TrimSpace(attempt.ActionKind) != "" {
		outbound.LogSendOutcome(s.state.Logger, outbound.SendLogContext{
			TargetLabel: buildCooldownTargetLabel(ctx, event, s.outboundSender),
		}, attempt, result, err)
	}
}

func (s *eventIngressService) waitOutboundLimit(ctx context.Context, request outbound.MessageLimitRequest) error {
	if s == nil || s.outboundLimiter == nil {
		return nil
	}
	return s.outboundLimiter.Wait(ctx, request)
}

func buildCooldownTargetLabel(ctx context.Context, event adapter.NormalizedEvent, sender outboundActionSender) string {
	targetType := strings.TrimSpace(event.ConversationType)
	targetID := strings.TrimSpace(event.ConversationID)
	targetName := ""
	actorID := ""
	actorNickname := ""

	switch targetType {
	case "group":
		targetName = strings.TrimSpace(event.TargetName)
	case "private":
		actorID = strings.TrimSpace(event.SenderID)
		actorNickname = strings.TrimSpace(event.ActorNickname)
	}

	var resolver outbound.TargetDisplayResolver
	if candidate, ok := any(sender).(outbound.TargetDisplayResolver); ok {
		resolver = candidate
	}

	return outbound.BuildTargetLabel(ctx, targetType, targetID, targetName, actorID, actorNickname, resolver)
}
