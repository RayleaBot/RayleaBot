package app

import (
	"strings"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

func (s *eventIngressService) logCommandPolicyRejection(event adapterintake.NormalizedEvent, verdict permission.Verdict, commandContext *commandPolicyContext) {
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
