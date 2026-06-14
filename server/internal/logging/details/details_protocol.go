package details

import "strings"

func NormalizeProtocol(protocol string, details map[string]any) map[string]any {
	normalized := sanitizeMap(CloneMap(details))
	switch strings.TrimSpace(protocol) {
	case "onebot11":
		return compactOneBot11LogDetails(normalized)
	default:
		return normalized
	}
}

func compactOneBot11LogDetails(details map[string]any) map[string]any {
	if len(details) == 0 {
		return map[string]any{}
	}

	sender, _ := details["sender"].(map[string]any)
	if sender == nil {
		sender = map[string]any{}
	}

	mergeField(sender, "user_id", details["sender_id"])
	mergeField(sender, "user_id", details["user_id"])
	mergeField(sender, "nickname", details["sender_nickname"])
	mergeField(sender, "card", details["sender_card"])
	mergeField(sender, "role", details["sender_role"])
	mergeField(sender, "title", details["sender_title"])

	if len(sender) > 0 {
		details["sender"] = sender
		delete(details, "sender_id")
		delete(details, "sender_nickname")
		delete(details, "sender_card")
		delete(details, "sender_role")
		delete(details, "sender_title")

		if valuesEqual(details["user_id"], sender["user_id"]) {
			delete(details, "user_id")
		}
	}

	if valuesEqual(details["time"], details["event_timestamp"]) {
		delete(details, "time")
	}
	if valuesEqual(details["group_id"], details["conversation_id"]) {
		delete(details, "group_id")
	}
	if valuesEqual(details["real_id"], details["message_id"]) {
		delete(details, "real_id")
	}
	if valuesEqual(details["message_seq"], details["message_id"]) {
		delete(details, "message_seq")
	}

	return details
}
