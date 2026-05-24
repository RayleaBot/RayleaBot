package logging

import (
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/textsafe"
)

type OneBotInboundMessageSummaryInput struct {
	SourceProtocol   string
	BotID            string
	EventType        string
	ConversationType string
	ConversationID   string
	SenderID         string
	TargetName       string
	ActorNickname    string
	PlainText        string
	PayloadFields    map[string]any
}

func OneBotInboundMessageSummary(input OneBotInboundMessageSummaryInput) (string, bool) {
	if strings.TrimSpace(input.SourceProtocol) != ProtocolOneBot11 {
		return "", false
	}

	messageText := summarizeOneBotInboundMessageText(input.PlainText)
	if messageText == "" {
		return "", false
	}

	botID := strings.TrimSpace(input.BotID)
	if botID == "" {
		return "", false
	}

	senderID := strings.TrimSpace(input.SenderID)
	if senderID == "" {
		return "", false
	}

	senderDisplay := oneBotSenderDisplay(input)
	if senderDisplay == "" {
		senderDisplay = senderID
	}

	switch strings.TrimSpace(input.EventType) {
	case "message.group":
		return fmt.Sprintf("%s: %s%s%s(%s): %s",
			botID,
			oneBotGroupDisplay(input),
			oneBotSenderTitle(input.PayloadFields),
			senderDisplay,
			senderID,
			messageText,
		), true
	case "message.private":
		return fmt.Sprintf("%s: %s(%s): %s", botID, senderDisplay, senderID, messageText), true
	default:
		return "", false
	}
}

func summarizeOneBotInboundMessageText(text string) string {
	text = strings.TrimSpace(textsafe.SanitizeString(text))
	if text == "" {
		return ""
	}
	return textsafe.TruncateRunes(text, 160, "...")
}

func oneBotGroupDisplay(input OneBotInboundMessageSummaryInput) string {
	groupID := strings.TrimSpace(input.ConversationID)
	groupName := strings.TrimSpace(textsafe.SanitizeString(input.TargetName))
	if groupName == "" {
		return fmt.Sprintf("[%s]", groupID)
	}
	return fmt.Sprintf("[%s(%s)]", groupName, groupID)
}

func oneBotSenderTitle(payloadFields map[string]any) string {
	onebot := oneBotPayload(payloadFields)
	if sender, ok := onebot["sender"].(map[string]any); ok {
		if title := strings.TrimSpace(textsafe.SanitizeString(fmt.Sprint(sender["title"]))); title != "" && title != "<nil>" {
			return fmt.Sprintf("[%s]", title)
		}
	}
	return ""
}

func oneBotSenderDisplay(input OneBotInboundMessageSummaryInput) string {
	onebot := oneBotPayload(input.PayloadFields)
	if sender, ok := onebot["sender"].(map[string]any); ok {
		card := strings.TrimSpace(textsafe.SanitizeString(fmt.Sprint(sender["card"])))
		if card == "<nil>" {
			card = ""
		}
		nickname := strings.TrimSpace(textsafe.SanitizeString(fmt.Sprint(sender["nickname"])))
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

	return strings.TrimSpace(textsafe.SanitizeString(input.ActorNickname))
}

func oneBotPayload(payloadFields map[string]any) map[string]any {
	if payloadFields == nil {
		return nil
	}
	onebot, _ := payloadFields["onebot"].(map[string]any)
	return onebot
}
