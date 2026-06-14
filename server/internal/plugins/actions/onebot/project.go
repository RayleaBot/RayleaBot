package onebot

import (
	"fmt"
	"strings"

	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

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
		return "", nil, &runtimemanager.Error{
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
		return "", nil, &runtimemanager.Error{
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
		return "", nil, &runtimemanager.Error{
			Code:    "plugin.protocol_violation",
			Message: "onebot action missing conversation_type",
		}
	}
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
