package bridge

import (
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

func bridgeEventSummary(action string, event adapter.NormalizedEvent) string {
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
		return summary
	}

	base := "adapter event"
	switch event.EventType {
	case "message.group":
		base = "group message"
	case "message.private":
		base = "private message"
	case "message_sent.group":
		base = "sent group message"
	case "message_sent.private":
		base = "sent private message"
	case "notice.member_increase":
		base = "group member increase notice"
	case "notice.member_decrease":
		base = "group member decrease notice"
	case "notice.group_admin":
		base = "group admin notice"
	case "notice.group_ban":
		base = "group ban notice"
	case "notice.group_recall":
		base = "group recall notice"
	case "notice.group_upload":
		base = "group upload notice"
	case "notice.group_card":
		base = "group card notice"
	case "notice.group_title":
		base = "group title notice"
	case "notice.group_essence":
		base = "group essence notice"
	case "notice.group_message_emoji_like":
		base = "group emoji reaction notice"
	case "notice.friend_add":
		base = "friend add notice"
	case "notice.friend_recall":
		base = "friend recall notice"
	case "notice.profile_like":
		base = "profile like notice"
	case "notice.poke":
		base = "poke notice"
	case "notice.poke_recall":
		base = "poke recall notice"
	case "notice.flash_file":
		base = "flash file notice"
	case "request.friend":
		base = "friend request"
	case "request.group":
		base = "group request"
	case "meta.heartbeat":
		base = "heartbeat event"
	case "meta.lifecycle":
		base = "lifecycle event"
	}

	summary := fmt.Sprintf("runtime bridge %s %s", action, base)
	if text := strings.TrimSpace(event.PlainText); text != "" {
		summary += ": " + summarizeBridgeText(text)
	}
	return summary
}

func commandPolicyRejectedSummary(rejection CommandPolicyRejection) string {
	commandName := strings.TrimSpace(rejection.CommandName)
	reasonSummary := strings.TrimSpace(rejection.ReasonSummary)
	if reasonSummary == "" {
		reasonSummary = strings.TrimSpace(rejection.Reason)
	}

	switch {
	case commandName == "" && reasonSummary == "":
		return "command rejected by command policy"
	case commandName == "":
		return fmt.Sprintf("command rejected by command policy: %s", reasonSummary)
	}

	if pluginID := strings.TrimSpace(rejection.PluginID); pluginID != "" {
		if reasonSummary == "" {
			return fmt.Sprintf("plugin %s command %s rejected by command policy", pluginID, commandName)
		}
		return fmt.Sprintf("plugin %s command %s rejected by command policy: %s", pluginID, commandName, reasonSummary)
	}
	if reasonSummary == "" {
		return fmt.Sprintf("command %s rejected by command policy", commandName)
	}
	return fmt.Sprintf("command %s rejected by command policy: %s", commandName, reasonSummary)
}

func summarizeBridgeText(text string) string {
	text = strings.TrimSpace(textsafe.SanitizeString(text))
	if text == "" {
		return ""
	}
	return textsafe.TruncateRunes(text, 160, "...")
}
