package app

import (
	"context"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

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
