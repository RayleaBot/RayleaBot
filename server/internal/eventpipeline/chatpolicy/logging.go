package chatpolicy

import (
	"strings"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

func (s *Service) logCommandPolicyRejection(event adapterintake.NormalizedEvent, verdict permission.Verdict, commandContext *commandPolicyContext) {
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
		return "发送者不在白名单中"
	case "permission.blacklisted":
		if strings.TrimSpace(verdict.Reason) == "群在黑名单中" {
			return "群在黑名单中"
		}
		return "用户在黑名单中"
	case "permission.denied":
		return "权限等级不足"
	case "platform.user_rate_limited":
		return "用户命令触发频率限制"
	case "platform.rate_limited":
		return "群命令触发频率限制"
	default:
		return strings.TrimSpace(verdict.Reason)
	}
}
