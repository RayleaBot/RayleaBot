package actions

import (
	"fmt"
	"strings"

	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func normalizeParams(raw map[string]any) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	params := make(map[string]any, len(raw))
	for key, value := range raw {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		switch normalizedKey {
		case "conversation_id":
			continue
		case "limit":
			params[normalizedKey] = normalizeNumericValue(value)
		case "duration_seconds":
			params["duration"] = normalizeNumericValue(value)
		case "emoji":
			params["emoji_id"] = value
		case "target_id", "user_id", "group_id", "message_id":
			params[normalizedKey] = apiValue(fmt.Sprint(value))
		default:
			params[normalizedKey] = value
		}
	}
	return params, nil
}

func requiredString(data map[string]any, key string) (string, error) {
	if len(data) == 0 {
		return "", &pluginruntime.Error{
			Code:    "plugin.protocol_violation",
			Message: fmt.Sprintf("onebot action missing %s", key),
		}
	}
	value, ok := data[key]
	if !ok {
		return "", &pluginruntime.Error{
			Code:    "plugin.protocol_violation",
			Message: fmt.Sprintf("onebot action missing %s", key),
		}
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return "", &pluginruntime.Error{
			Code:    "plugin.protocol_violation",
			Message: fmt.Sprintf("onebot action missing %s", key),
		}
	}
	return text, nil
}

func optionalString(data map[string]any, key string) (string, bool) {
	if len(data) == 0 {
		return "", false
	}
	value, ok := data[key]
	if !ok {
		return "", false
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return "", false
	}
	return text, true
}

func normalizeNumericValue(value any) any {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	default:
		return value
	}
}

func apiValue(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	for _, char := range raw {
		if char < '0' || char > '9' {
			return raw
		}
	}
	return raw
}

func defaultResult(collectionKey string, result any) map[string]any {
	switch typed := result.(type) {
	case nil:
		return map[string]any{"ok": true}
	case map[string]any:
		if len(typed) == 0 {
			return map[string]any{"ok": true}
		}
		return typed
	case []any:
		return map[string]any{collectionKeyOrDefault(collectionKey): typed}
	default:
		return map[string]any{"value": typed}
	}
}

func collectionKeyOrDefault(collectionKey string) string {
	if strings.TrimSpace(collectionKey) == "" {
		return "items"
	}
	return collectionKey
}

func projectMessageHistoryGet(raw map[string]any) (string, map[string]any, error) {
	conversationType, err := requiredString(raw, "conversation_type")
	if err != nil {
		return "", nil, err
	}
	conversationID, err := requiredString(raw, "conversation_id")
	if err != nil {
		return "", nil, err
	}
	historyParams := map[string]any{}
	if limit, ok := raw["limit"]; ok {
		historyParams["limit"] = limit
	}
	switch conversationType {
	case "group":
		historyParams["group_id"] = apiValue(conversationID)
		return "get_group_msg_history", historyParams, nil
	case "private":
		historyParams["user_id"] = apiValue(conversationID)
		return "get_friend_msg_history", historyParams, nil
	default:
		return "", nil, &pluginruntime.Error{
			Code:    "plugin.protocol_violation",
			Message: "onebot action missing conversation_type",
		}
	}
}

func projectMessageForwardGet(raw map[string]any) (string, map[string]any, error) {
	params, err := normalizeParams(raw)
	if err != nil {
		return "", nil, err
	}
	if _, err := requiredString(raw, "message_id"); err != nil {
		if _, altErr := requiredString(raw, "forward_id"); altErr != nil {
			return "", nil, err
		}
	}
	if value, ok := params["message_id"]; !ok || strings.TrimSpace(fmt.Sprint(value)) == "" {
		params["message_id"] = params["forward_id"]
	}
	delete(params, "forward_id")
	return "get_forward_msg", params, nil
}

func projectMessageForwardSend(raw map[string]any) (string, map[string]any, error) {
	params, err := normalizeParams(raw)
	if err != nil {
		return "", nil, err
	}
	targetType, err := requiredString(raw, "target_type")
	if err != nil {
		return "", nil, err
	}
	targetID, err := requiredString(raw, "target_id")
	if err != nil {
		return "", nil, err
	}
	switch targetType {
	case "group":
		params["group_id"] = apiValue(targetID)
		delete(params, "target_id")
		delete(params, "target_type")
		return "send_group_forward_msg", params, nil
	case "private":
		params["user_id"] = apiValue(targetID)
		delete(params, "target_id")
		delete(params, "target_type")
		return "send_private_forward_msg", params, nil
	default:
		return "", nil, &pluginruntime.Error{
			Code:    "plugin.protocol_violation",
			Message: "onebot action missing target_type",
		}
	}
}

func projectMessageReadMark(raw map[string]any) (string, map[string]any, error) {
	if messageID, ok := optionalString(raw, "message_id"); ok {
		return "mark_msg_as_read", map[string]any{"message_id": apiValue(messageID)}, nil
	}
	targetType, err := requiredString(raw, "conversation_type")
	if err != nil {
		return "", nil, err
	}
	targetID, err := requiredString(raw, "conversation_id")
	if err != nil {
		return "", nil, err
	}
	switch targetType {
	case "group":
		return "mark_group_msg_as_read", map[string]any{"group_id": apiValue(targetID)}, nil
	case "private":
		return "mark_private_msg_as_read", map[string]any{"user_id": apiValue(targetID)}, nil
	default:
		return "", nil, &pluginruntime.Error{
			Code:    "plugin.protocol_violation",
			Message: "onebot action missing conversation_type",
		}
	}
}

func projectGroupMemberGet(raw map[string]any) (string, map[string]any, error) {
	if _, err := requiredString(raw, "group_id"); err != nil {
		return "", nil, err
	}
	if _, err := requiredString(raw, "user_id"); err != nil {
		return "", nil, err
	}
	params, err := normalizeParams(raw)
	if err != nil {
		return "", nil, err
	}
	params["no_cache"] = true
	return "get_group_member_info", params, nil
}

func projectGroupBanSet(raw map[string]any) (string, map[string]any, error) {
	params, err := normalizeParams(raw)
	if err != nil {
		return "", nil, err
	}
	if whole, ok := raw["whole_group"].(bool); ok && whole {
		delete(params, "user_id")
		delete(params, "duration_seconds")
		delete(params, "duration")
		return "set_group_whole_ban", params, nil
	}
	return "set_group_ban", params, nil
}

func projectGroupFilesList(raw map[string]any) (string, map[string]any, error) {
	params, err := normalizeParams(raw)
	if err != nil {
		return "", nil, err
	}
	if folderID, ok := optionalString(raw, "folder_id"); ok {
		params["folder_id"] = folderID
		return "get_group_files_by_folder", params, nil
	}
	return "get_group_root_files", params, nil
}

func projectGroupFilesDelete(raw map[string]any) (string, map[string]any, error) {
	params, err := normalizeParams(raw)
	if err != nil {
		return "", nil, err
	}
	if folderID, ok := optionalString(raw, "folder_id"); ok && folderID != "" {
		return "delete_group_folder", map[string]any{
			"group_id":  params["group_id"],
			"folder_id": folderID,
		}, nil
	}
	return "delete_group_file", params, nil
}
