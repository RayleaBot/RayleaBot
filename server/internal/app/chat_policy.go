package app

import (
	"context"
	"strings"
	"time"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/config"
	"rayleabot/server/internal/permission"
	"rayleabot/server/internal/plugins"
)

const (
	defaultUserCommandRateLimit  = "10/60s"
	defaultGroupCommandRateLimit = "30/60s"
	cooldownReplyText            = "命令触发冷却，请稍后再试。"
)

type outboundActionSender interface {
	SendMessage(context.Context, adapter.OutboundMessageSend) (adapter.SendMessageResult, error)
	SendReply(context.Context, adapter.OutboundMessageReply) (adapter.SendMessageResult, error)
	SendImage(context.Context, adapter.OutboundMessageSendImage) (adapter.SendMessageResult, error)
}

func newPermissionChecker(cfg config.Config, repo permission.BlacklistRepository) *permission.Checker {
	userLimit := parseCooldownRateLimit(defaultUserCommandRateLimit)
	groupLimit := parseCooldownRateLimit(defaultGroupCommandRateLimit)
	if cfg.Cooldown != nil {
		userLimit = parseCooldownRateLimitWithFallback(cfg.Cooldown.UserCommandRateLimit, defaultUserCommandRateLimit)
		groupLimit = parseCooldownRateLimitWithFallback(cfg.Cooldown.GroupCommandRateLimit, defaultGroupCommandRateLimit)
	}

	return permission.NewChecker(permission.CheckerConfig{
		SuperAdmins:  append([]string(nil), cfg.Auth.SuperAdmins...),
		DefaultLevel: commandPermissionDefaultLevel(cfg),
	}, repo, permission.NewCooldownTracker(userLimit, groupLimit))
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
	switch strings.TrimSpace(cfg.Auth.DefaultLevel) {
	case "super_admin", "group_admin", "everyone":
		return strings.TrimSpace(cfg.Auth.DefaultLevel)
	default:
		return "everyone"
	}
}

func cooldownReplyEnabled(cfg config.Config) bool {
	if cfg.Cooldown == nil {
		return true
	}
	return cfg.Cooldown.CooldownReply
}

func (a *App) applyChatPolicy(ctx context.Context, event adapter.NormalizedEvent) (adapter.NormalizedEvent, bool) {
	enriched := a.enrichCommandEvent(event)
	if a == nil || a.permissionChecker == nil || !shouldEvaluateChatPolicy(enriched) {
		return enriched, true
	}

	verdict := a.permissionChecker.Check(
		ctx,
		strings.TrimSpace(enriched.SenderID),
		strings.TrimSpace(enriched.ActorRole),
		commandGroupID(enriched),
		a.commandInfoForEvent(enriched),
	)
	if verdict.Allowed {
		return enriched, true
	}

	if (verdict.ErrorCode == "platform.user_rate_limited" || verdict.ErrorCode == "platform.rate_limited") && cooldownReplyEnabled(a.Config) {
		a.sendCooldownReply(enriched)
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

func (a *App) commandInfoForEvent(event adapter.NormalizedEvent) *permission.CommandInfo {
	commandName := commandNameFromEvent(event)
	if commandName == "" {
		return nil
	}

	requiredLevel := "everyone"
	matched := false
	if a != nil && a.Plugins != nil {
		for _, snapshot := range a.Plugins.List() {
			if !pluginParticipatesInCommandPolicy(snapshot) {
				continue
			}
			for _, command := range snapshot.Commands {
				if !commandMatches(command, commandName) {
					continue
				}
				matched = true
				level := effectiveCommandPermissionLevel(command.Permission, a.Config)
				if commandPermissionRank(level) > commandPermissionRank(requiredLevel) {
					requiredLevel = level
				}
				break
			}
		}
	}

	if !matched {
		return &permission.CommandInfo{Permission: "everyone"}
	}
	return &permission.CommandInfo{Permission: requiredLevel}
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

func (a *App) sendCooldownReply(event adapter.NormalizedEvent) {
	if a == nil || a.outboundSender == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err error
	switch strings.TrimSpace(event.ConversationType) {
	case "group":
		if messageID := strings.TrimSpace(event.MessageID); messageID != "" {
			_, err = a.outboundSender.SendReply(ctx, adapter.OutboundMessageReply{
				TargetType:       "group",
				TargetID:         strings.TrimSpace(event.ConversationID),
				ReplyToMessageID: messageID,
				Text:             cooldownReplyText,
			})
			break
		}
		fallthrough
	case "private":
		if targetID := strings.TrimSpace(event.ConversationID); targetID != "" {
			_, err = a.outboundSender.SendMessage(ctx, adapter.OutboundMessageSend{
				TargetType: strings.TrimSpace(event.ConversationType),
				TargetID:   targetID,
				Text:       cooldownReplyText,
			})
		}
	default:
		return
	}

	if err != nil && a.Logger != nil {
		a.Logger.Warn(
			"failed to send cooldown reply",
			"component", "app",
			"conversation_type", event.ConversationType,
			"conversation_id", event.ConversationID,
			"err", err.Error(),
		)
	}
}
