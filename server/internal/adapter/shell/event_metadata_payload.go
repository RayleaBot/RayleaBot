package shell

import (
	"strings"

	adapterapi "github.com/RayleaBot/RayleaBot/server/internal/adapter/api"
	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
)

func groupNameFromPayload(payload map[string]any) string {
	if len(payload) == 0 {
		return ""
	}
	if groupName := payloadStringValue(payload["group_name"]); groupName != "" {
		return groupName
	}
	onebot := cloneOptionalMap(payload["onebot"])
	return payloadStringValue(onebot["group_name"])
}

func cloneNormalizedEvent(event adapterintake.NormalizedEvent) adapterintake.NormalizedEvent {
	cloned := event
	cloned.PayloadFields = cloneEventMap(event.PayloadFields)
	return cloned
}

func cloneEventMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = cloneEventValue(value)
	}
	return cloned
}

func cloneEventValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneEventMap(typed)
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, cloneEventValue(item))
		}
		return items
	default:
		return value
	}
}

func unifiedSenderPayload(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return map[string]any{}
	}

	sender := cloneOptionalMap(payload["sender"])
	onebot := cloneOptionalMap(payload["onebot"])
	onebotSender := cloneOptionalMap(onebot["sender"])

	if len(sender) == 0 {
		sender = onebotSender
	} else {
		mergeSenderFields(sender, onebotSender)
	}

	return sender
}

func cloneOptionalMap(value any) map[string]any {
	typed, _ := value.(map[string]any)
	return cloneEventMap(typed)
}

func mergeSenderFields(target map[string]any, source map[string]any) {
	if len(target) == 0 || len(source) == 0 {
		if len(target) == 0 && len(source) > 0 {
			for key, value := range source {
				target[key] = value
			}
		}
		return
	}

	for _, key := range []string{"user_id", "nickname", "card", "role", "title"} {
		if payloadStringValue(target[key]) != "" {
			continue
		}
		if payloadStringValue(source[key]) == "" {
			continue
		}
		target[key] = source[key]
	}
}

func syncSenderPayload(payload map[string]any, sender map[string]any) {
	if len(payload) == 0 || len(sender) == 0 {
		return
	}

	payload["sender"] = sender
	onebot := cloneOptionalMap(payload["onebot"])
	if len(onebot) == 0 {
		onebot = map[string]any{}
	}
	onebot["sender"] = sender
	payload["onebot"] = onebot
}

func mergeGroupMemberInfo(sender map[string]any, info adapterapi.GroupMemberInfo) {
	if payloadStringValue(sender["card"]) == "" && strings.TrimSpace(info.Card) != "" {
		sender["card"] = info.Card
	}
	if payloadStringValue(sender["nickname"]) == "" && strings.TrimSpace(info.Nickname) != "" {
		sender["nickname"] = info.Nickname
	}
	if payloadStringValue(sender["role"]) == "" && strings.TrimSpace(info.Role) != "" {
		sender["role"] = info.Role
	}
	if payloadStringValue(sender["title"]) == "" && strings.TrimSpace(info.Title) != "" {
		sender["title"] = info.Title
	}
}

func mergeStrangerInfo(sender map[string]any, info adapterapi.StrangerInfo) {
	if payloadStringValue(sender["nickname"]) == "" && strings.TrimSpace(info.Nickname) != "" {
		sender["nickname"] = info.Nickname
	}
}

func senderDisplayName(sender map[string]any) string {
	card := payloadStringValue(sender["card"])
	nickname := payloadStringValue(sender["nickname"])

	switch {
	case card != "" && nickname != "" && card != nickname:
		return card + "/" + nickname
	case card != "":
		return card
	case nickname != "":
		return nickname
	default:
		return ""
	}
}

func senderPrimaryName(sender map[string]any) string {
	card := payloadStringValue(sender["card"])
	if card != "" {
		return card
	}
	return payloadStringValue(sender["nickname"])
}

func payloadStringValue(value any) string {
	if value == nil {
		return ""
	}
	valueString := strings.TrimSpace(extractStringValue(value))
	if valueString == "<nil>" {
		return ""
	}
	return valueString
}
