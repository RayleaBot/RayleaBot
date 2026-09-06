package bridge

import (
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/redact"
)

func bridgeEventSummary(action string, event chatevent.NormalizedEvent) string {
	action = strings.TrimSpace(action)
	if summary, ok := logging.OneBotInboundMessageSummary(logging.OneBotInboundMessageSummaryInput{
		SourceProtocol:   event.SourceProtocol,
		BotID:            event.BotID,
		EventType:        event.EventType,
		ConversationType: event.ConversationType,
		ConversationID:   event.ConversationID,
		SenderID:         event.SenderID,
		TargetName:       event.TargetName,
		ActorNickname:    event.ActorNickname,
		PlainText:        event.PlainText,
		PayloadFields:    event.PayloadFields,
	}); ok {
		switch action {
		case "ignored":
			return summary + "；没有匹配的插件。"
		case "queued for dispatcher":
			return summary
		case "failed to queue for dispatcher":
			return summary + "；插件暂时无法接收这条消息。"
		default:
			return summary
		}
	}

	base := "消息平台通知"
	switch event.EventType {
	case "message.group":
		base = "群消息"
	case "message.private":
		base = "私聊消息"
	case "message_sent.group":
		base = "群消息发送回执"
	case "message_sent.private":
		base = "私聊消息发送回执"
	case "notice.member_increase":
		base = "群成员增加通知"
	case "notice.member_decrease":
		base = "群成员减少通知"
	case "notice.group_admin":
		base = "群管理员变更通知"
	case "notice.group_ban":
		base = "群禁言通知"
	case "notice.group_recall":
		base = "群消息撤回通知"
	case "notice.group_upload":
		base = "群文件上传通知"
	case "notice.group_card":
		base = "群名片变更通知"
	case "notice.group_title":
		base = "群头衔变更通知"
	case "notice.group_essence":
		base = "群精华消息通知"
	case "notice.group_message_emoji_like":
		base = "群表情回应通知"
	case "notice.friend_add":
		base = "好友添加通知"
	case "notice.friend_recall":
		base = "好友消息撤回通知"
	case "notice.profile_like":
		base = "资料卡点赞通知"
	case "notice.poke":
		base = "戳一戳通知"
	case "notice.poke_recall":
		base = "戳一戳撤回通知"
	case "notice.flash_file":
		base = "闪传文件通知"
	case "request.friend":
		base = "好友请求"
	case "request.group":
		base = "群请求"
	case "meta.heartbeat":
		base = "连接状态检查"
	case "meta.lifecycle":
		base = "连接状态通知"
	}

	actionLabel := map[string]string{
		"ignored":                        "已忽略",
		"queued for dispatcher":          "收到",
		"failed to queue for dispatcher": "无法接收",
	}[action]
	if actionLabel == "" {
		actionLabel = strings.TrimSpace(action)
	}
	summary := fmt.Sprintf("%s%s", actionLabel, base)
	if text := strings.TrimSpace(event.PlainText); text != "" {
		summary += "：" + summarizeBridgeText(text)
	}
	if action == "failed to queue for dispatcher" {
		summary += "；请检查插件状态。"
	}
	return summary
}

func commandPolicyRejectedSummary(rejection CommandPolicyRejection) string {
	commandName := strings.TrimSpace(rejection.CommandName)
	reasonSummary := strings.TrimSpace(rejection.ReasonSummary)
	if reasonSummary == "" {
		reasonSummary = strings.TrimSpace(rejection.Reason)
	}
	reasonSummary = commandPolicyReasonLabel(reasonSummary)

	switch {
	case commandName == "" && reasonSummary == "":
		return "命令未执行"
	case commandName == "":
		return fmt.Sprintf("命令未执行：%s", reasonSummary)
	}

	if pluginID := strings.TrimSpace(rejection.PluginID); pluginID != "" {
		if reasonSummary == "" {
			return fmt.Sprintf("插件 %s 的命令 %s 未执行", pluginID, commandName)
		}
		return fmt.Sprintf("插件 %s 的命令 %s 未执行：%s", pluginID, commandName, reasonSummary)
	}
	if reasonSummary == "" {
		return fmt.Sprintf("命令 %s 未执行", commandName)
	}
	return fmt.Sprintf("命令 %s 未执行：%s", commandName, reasonSummary)
}

func commandPolicyReasonLabel(reason string) string {
	switch strings.TrimSpace(reason) {
	case "actor is not whitelisted", "sender is not whitelisted":
		return "发送者不在白名单中"
	case "user is blacklisted":
		return "用户在黑名单中"
	case "group is blacklisted":
		return "群在黑名单中"
	case "insufficient permission level":
		return "权限等级不足"
	case "user command rate limited":
		return "用户命令触发频率限制"
	case "group command rate limited":
		return "群命令触发频率限制"
	default:
		return strings.TrimSpace(reason)
	}
}

func summarizeBridgeText(text string) string {
	text = strings.TrimSpace(redact.SanitizeString(text))
	if text == "" {
		return ""
	}
	return redact.TruncateRunes(text, 160, "...")
}
