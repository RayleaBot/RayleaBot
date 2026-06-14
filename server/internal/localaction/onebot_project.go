package localaction

import (
	"fmt"
	"strings"

	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/runtime/manager"
)

func projectOneBotMessageHistoryGet(raw map[string]any) (string, map[string]any, error) {
	conversationType, err := requiredActionString(raw, "conversation_type")
	if err != nil {
		return "", nil, err
	}
	conversationID, err := requiredActionString(raw, "conversation_id")
	if err != nil {
		return "", nil, err
	}
	historyParams := map[string]any{}
	if limit, ok := raw["limit"]; ok {
		historyParams["limit"] = limit
	}
	switch conversationType {
	case "group":
		historyParams["group_id"] = oneBotAPIValue(conversationID)
		return "get_group_msg_history", historyParams, nil
	case "private":
		historyParams["user_id"] = oneBotAPIValue(conversationID)
		return "get_friend_msg_history", historyParams, nil
	default:
		return "", nil, &runtimemanager.Error{
			Code:    "plugin.protocol_violation",
			Message: "onebot action missing conversation_type",
		}
	}
}

func projectOneBotMessageForwardGet(raw map[string]any) (string, map[string]any, error) {
	params, err := normalizeActionParams(raw)
	if err != nil {
		return "", nil, err
	}
	if _, err := requiredActionString(raw, "message_id"); err != nil {
		if _, altErr := requiredActionString(raw, "forward_id"); altErr != nil {
			return "", nil, err
		}
	}
	if value, ok := params["message_id"]; !ok || strings.TrimSpace(fmt.Sprint(value)) == "" {
		params["message_id"] = params["forward_id"]
	}
	delete(params, "forward_id")
	return "get_forward_msg", params, nil
}

func projectOneBotMessageForwardSend(raw map[string]any) (string, map[string]any, error) {
	params, err := normalizeActionParams(raw)
	if err != nil {
		return "", nil, err
	}
	targetType, err := requiredActionString(raw, "target_type")
	if err != nil {
		return "", nil, err
	}
	targetID, err := requiredActionString(raw, "target_id")
	if err != nil {
		return "", nil, err
	}
	switch targetType {
	case "group":
		params["group_id"] = oneBotAPIValue(targetID)
		delete(params, "target_id")
		delete(params, "target_type")
		return "send_group_forward_msg", params, nil
	case "private":
		params["user_id"] = oneBotAPIValue(targetID)
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

func projectOneBotMessageReadMark(raw map[string]any) (string, map[string]any, error) {
	if messageID, ok := optionalActionString(raw, "message_id"); ok {
		return "mark_msg_as_read", map[string]any{"message_id": oneBotAPIValue(messageID)}, nil
	}
	targetType, err := requiredActionString(raw, "conversation_type")
	if err != nil {
		return "", nil, err
	}
	targetID, err := requiredActionString(raw, "conversation_id")
	if err != nil {
		return "", nil, err
	}
	switch targetType {
	case "group":
		return "mark_group_msg_as_read", map[string]any{"group_id": oneBotAPIValue(targetID)}, nil
	case "private":
		return "mark_private_msg_as_read", map[string]any{"user_id": oneBotAPIValue(targetID)}, nil
	default:
		return "", nil, &runtimemanager.Error{
			Code:    "plugin.protocol_violation",
			Message: "onebot action missing conversation_type",
		}
	}
}

func projectOneBotGroupBanSet(raw map[string]any) (string, map[string]any, error) {
	params, err := normalizeActionParams(raw)
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

func projectOneBotGroupFilesList(raw map[string]any) (string, map[string]any, error) {
	params, err := normalizeActionParams(raw)
	if err != nil {
		return "", nil, err
	}
	if folderID, ok := optionalActionString(raw, "folder_id"); ok {
		params["folder_id"] = folderID
		return "get_group_files_by_folder", params, nil
	}
	return "get_group_root_files", params, nil
}

func projectOneBotGroupFilesDelete(raw map[string]any) (string, map[string]any, error) {
	params, err := normalizeActionParams(raw)
	if err != nil {
		return "", nil, err
	}
	if folderID, ok := optionalActionString(raw, "folder_id"); ok && folderID != "" {
		return "delete_group_folder", map[string]any{
			"group_id":  params["group_id"],
			"folder_id": folderID,
		}, nil
	}
	return "delete_group_file", params, nil
}
