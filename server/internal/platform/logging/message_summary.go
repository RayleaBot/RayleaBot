package logging

import (
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/redact"
)

// messageSummaryTextLimit bounds the message text a log body carries. The
// full text stays in the structured plain_text detail.
const messageSummaryTextLimit = 160

// InboundMessageSummaryInput names the parties of one received message. The
// bot nickname is optional: an adapter that has not learned its own login yet
// leaves it empty, and the body falls back to the bare account id.
type InboundMessageSummaryInput struct {
	SourceProtocol   string
	BotID            string
	BotNickname      string
	EventType        string
	ConversationType string
	ConversationID   string
	SenderID         string
	TargetName       string
	ActorNickname    string
	PlainText        string
	PayloadFields    map[string]any
}

// OutboundMessageSummaryInput names the parties of one sent message. The
// target label arrives already formatted, because only the outbound pipeline
// can ask the owning adapter for conversation names.
type OutboundMessageSummaryInput struct {
	BotID       string
	BotNickname string
	TargetLabel string
	PlainText   string
}

// AccountLabel renders one chat account as "昵称(账号)", falling back to
// whichever half is known.
func AccountLabel(id, nickname string) string {
	id = strings.TrimSpace(id)
	nickname = strings.TrimSpace(redact.SanitizeString(nickname))
	switch {
	case nickname != "" && id != "":
		return fmt.Sprintf("%s(%s)", nickname, id)
	case nickname != "":
		return nickname
	default:
		return id
	}
}

// InboundMessageSummary renders the body of a received-message log line as
// "机器人昵称(账号): [群名(群号)][头衔]发送者(账号): 内容". It reports false for
// events it cannot describe this way, so the caller can fall back to a
// generic body.
func InboundMessageSummary(input InboundMessageSummaryInput) (string, bool) {
	if !IsSupportedProtocol(input.SourceProtocol) {
		return "", false
	}

	messageText := SummarizeMessageText(input.PlainText)
	if messageText == "" {
		return "", false
	}

	botID := strings.TrimSpace(input.BotID)
	if botID == "" {
		return "", false
	}
	botLabel := AccountLabel(botID, input.BotNickname)

	senderID := strings.TrimSpace(input.SenderID)
	if senderID == "" {
		return "", false
	}

	senderDisplay := inboundSenderDisplay(input)
	if senderDisplay == "" {
		senderDisplay = senderID
	}

	switch strings.TrimSpace(input.EventType) {
	case "message.group":
		return fmt.Sprintf("%s: %s%s%s(%s): %s",
			botLabel,
			inboundGroupDisplay(input),
			oneBotSenderTitle(input.PayloadFields),
			senderDisplay,
			senderID,
			messageText,
		), true
	case "message.private":
		return fmt.Sprintf("%s: %s(%s): %s", botLabel, senderDisplay, senderID, messageText), true
	default:
		return "", false
	}
}

// OutboundMessageSummary renders the body of a sent-message log line as
// "机器人昵称(账号) -> 目标: 内容". A party the caller could not name is left
// out rather than rendered as an empty bracket.
func OutboundMessageSummary(input OutboundMessageSummaryInput) string {
	messageText := SummarizeMessageText(input.PlainText)
	if messageText == "" {
		messageText = "[empty message]"
	}

	parties := make([]string, 0, 2)
	if botLabel := AccountLabel(input.BotID, input.BotNickname); botLabel != "" {
		parties = append(parties, botLabel)
	}
	if targetLabel := strings.TrimSpace(redact.SanitizeString(input.TargetLabel)); targetLabel != "" {
		parties = append(parties, targetLabel)
	}
	if len(parties) == 0 {
		return messageText
	}
	return strings.Join(parties, " -> ") + ": " + messageText
}

// SummarizeMessageText sanitizes message text for a log body and truncates it
// so one long message cannot flood the log list.
func SummarizeMessageText(text string) string {
	text = strings.TrimSpace(redact.SanitizeString(text))
	if text == "" {
		return ""
	}
	return redact.TruncateRunes(text, messageSummaryTextLimit, "...")
}

func inboundGroupDisplay(input InboundMessageSummaryInput) string {
	groupID := strings.TrimSpace(input.ConversationID)
	groupName := strings.TrimSpace(redact.SanitizeString(input.TargetName))
	if groupName == "" {
		return fmt.Sprintf("[%s]", groupID)
	}
	return fmt.Sprintf("[%s(%s)]", groupName, groupID)
}

func oneBotSenderTitle(payloadFields map[string]any) string {
	onebot := oneBotPayload(payloadFields)
	if sender, ok := onebot["sender"].(map[string]any); ok {
		if title := strings.TrimSpace(redact.SanitizeString(fmt.Sprint(sender["title"]))); title != "" && title != "<nil>" {
			return fmt.Sprintf("[%s]", title)
		}
	}
	return ""
}

// inboundSenderDisplay prefers the OneBot sender block, which carries the
// group card, and otherwise uses the nickname the adapter normalized.
func inboundSenderDisplay(input InboundMessageSummaryInput) string {
	onebot := oneBotPayload(input.PayloadFields)
	if sender, ok := onebot["sender"].(map[string]any); ok {
		card := strings.TrimSpace(redact.SanitizeString(fmt.Sprint(sender["card"])))
		if card == "<nil>" {
			card = ""
		}
		nickname := strings.TrimSpace(redact.SanitizeString(fmt.Sprint(sender["nickname"])))
		if nickname == "<nil>" {
			nickname = ""
		}

		switch {
		case card != "" && nickname != "" && card != nickname:
			return card + "/" + nickname
		case card != "":
			return card
		case nickname != "":
			return nickname
		}
	}

	return strings.TrimSpace(redact.SanitizeString(input.ActorNickname))
}

func oneBotPayload(payloadFields map[string]any) map[string]any {
	if payloadFields == nil {
		return nil
	}
	onebot, _ := payloadFields["onebot"].(map[string]any)
	return onebot
}
