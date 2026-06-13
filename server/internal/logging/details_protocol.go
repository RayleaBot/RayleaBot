package logging

import "strings"

func normalizeProtocolDetails(protocol string, details map[string]any) map[string]any {
	normalized := sanitizeDetailsMap(cloneDetailsMap(details))
	switch strings.TrimSpace(protocol) {
	case ProtocolOneBot11:
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

	mergeDetailField(sender, "user_id", details["sender_id"])
	mergeDetailField(sender, "user_id", details["user_id"])
	mergeDetailField(sender, "nickname", details["sender_nickname"])
	mergeDetailField(sender, "card", details["sender_card"])
	mergeDetailField(sender, "role", details["sender_role"])
	mergeDetailField(sender, "title", details["sender_title"])

	if len(sender) > 0 {
		details["sender"] = sender
		delete(details, "sender_id")
		delete(details, "sender_nickname")
		delete(details, "sender_card")
		delete(details, "sender_role")
		delete(details, "sender_title")

		if detailValuesEqual(details["user_id"], sender["user_id"]) {
			delete(details, "user_id")
		}
	}

	if detailValuesEqual(details["time"], details["event_timestamp"]) {
		delete(details, "time")
	}
	if detailValuesEqual(details["group_id"], details["conversation_id"]) {
		delete(details, "group_id")
	}
	if detailValuesEqual(details["real_id"], details["message_id"]) {
		delete(details, "real_id")
	}
	if detailValuesEqual(details["message_seq"], details["message_id"]) {
		delete(details, "message_seq")
	}

	return details
}
