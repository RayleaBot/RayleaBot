package localaction

import (
	"context"
	"fmt"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func runtimeIsOneBotLocalAction(kind string) bool {
	switch kind {
	case
		"message.get",
		"message.delete",
		"message.history.get",
		"message.forward.get",
		"message.forward.send",
		"message.read.mark",
		"friend.request.handle",
		"friend.list",
		"friend.remark.set",
		"user.info.get",
		"user.like.send",
		"group.list",
		"group.info.get",
		"group.member.get",
		"group.member.list",
		"group.request.handle",
		"group.leave",
		"group.admin.set",
		"group.ban.set",
		"group.card.set",
		"group.title.set",
		"group.name.set",
		"group.announcement.list",
		"group.announcement.create",
		"group.announcement.delete",
		"group.essence.list",
		"group.essence.set",
		"group.essence.unset",
		"group.honor.get",
		"group.todo.set",
		"file.get",
		"file.download",
		"file.group.upload",
		"file.private.upload",
		"file.group.url.get",
		"file.private.url.get",
		"file.group.fs.info",
		"file.group.fs.list",
		"file.group.fs.mkdir",
		"file.group.fs.delete",
		"reaction.set",
		"reaction.list",
		"poke.send":
		return true
	default:
		return false
	}
}

func runtimeIsProviderExtensionAction(kind string) bool {
	switch kind {
	case
		"provider.napcat.message_emoji.like.set",
		"provider.napcat.group.sign.set",
		"provider.luckylillia.friend_groups.get":
		return true
	default:
		return false
	}
}

func (s *Service) executeOneBotLocalAction(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, action.Kind) {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: action.Kind + " capability is not granted",
		}
	}

	if s.adapter == nil {
		return nil, &runtime.Error{
			Code:    "adapter.transport_not_implemented",
			Message: "OneBot adapter 不可用",
		}
	}

	if runtimeIsProviderExtensionAction(action.Kind) {
		return s.executeOneBotProviderAction(ctx, action)
	}

	apiAction, params, err := projectOneBotGenericAction(action)
	if err != nil {
		return nil, err
	}

	result, callErr := s.adapter.CallAPIAny(ctx, apiAction, params)
	if callErr != nil {
		return nil, toRuntimeActionError(callErr)
	}
	return projectOneBotActionResult(action.Kind, result), nil
}

func toRuntimeActionError(err error) error {
	if err == nil {
		return nil
	}
	if adapterErr, ok := err.(*adapter.Error); ok {
		return &runtime.Error{
			Code:    adapterErr.Code,
			Message: adapterErr.Message,
		}
	}
	return &runtime.Error{
		Code:    "adapter.transport_not_implemented",
		Message: err.Error(),
	}
}

func (s *Service) executeOneBotProviderAction(ctx context.Context, action runtime.Action) (map[string]any, error) {
	provider := s.adapter.Snapshot().DetectedProvider()

	var (
		requiredProvider string
		apiAction        string
		err              error
	)
	switch action.Kind {
	case "provider.napcat.message_emoji.like.set":
		requiredProvider = "napcat"
		apiAction = "set_msg_emoji_like"
	case "provider.napcat.group.sign.set":
		requiredProvider = "napcat"
		apiAction = "set_group_sign"
	case "provider.luckylillia.friend_groups.get":
		requiredProvider = "luckylillia"
		apiAction = "get_grouped_friend_list"
	default:
		return nil, &runtime.Error{
			Code:    "adapter.provider_extension_not_supported",
			Message: "当前 provider 扩展动作尚未实现",
		}
	}

	if provider != requiredProvider {
		return nil, &runtime.Error{
			Code:    "adapter.provider_extension_not_supported",
			Message: "当前 provider 不支持该扩展动作",
		}
	}

	params, err := normalizeActionParams(action.RawData)
	if err != nil {
		return nil, err
	}
	result, callErr := s.adapter.CallAPIAny(ctx, apiAction, params)
	if callErr != nil {
		return nil, toRuntimeActionError(callErr)
	}
	return projectOneBotActionResult(action.Kind, result), nil
}

func projectOneBotGenericAction(action runtime.Action) (string, map[string]any, error) {
	params, err := normalizeActionParams(action.RawData)
	if err != nil {
		return "", nil, err
	}

	switch action.Kind {
	case "message.get":
		if _, err := requiredActionString(action.RawData, "message_id"); err != nil {
			return "", nil, err
		}
		return "get_msg", params, nil
	case "message.delete":
		if _, err := requiredActionString(action.RawData, "message_id"); err != nil {
			return "", nil, err
		}
		return "delete_msg", params, nil
	case "message.history.get":
		conversationType, err := requiredActionString(action.RawData, "conversation_type")
		if err != nil {
			return "", nil, err
		}
		conversationID, err := requiredActionString(action.RawData, "conversation_id")
		if err != nil {
			return "", nil, err
		}
		historyParams := map[string]any{}
		if limit, ok := action.RawData["limit"]; ok {
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
			return "", nil, &runtime.Error{
				Code:    "plugin.protocol_violation",
				Message: "onebot action missing conversation_type",
			}
		}
	case "message.forward.get":
		if _, err := requiredActionString(action.RawData, "message_id"); err != nil {
			if _, altErr := requiredActionString(action.RawData, "forward_id"); altErr != nil {
				return "", nil, err
			}
		}
		if value, ok := params["message_id"]; !ok || strings.TrimSpace(fmt.Sprint(value)) == "" {
			params["message_id"] = params["forward_id"]
		}
		delete(params, "forward_id")
		return "get_forward_msg", params, nil
	case "message.forward.send":
		targetType, err := requiredActionString(action.RawData, "target_type")
		if err != nil {
			return "", nil, err
		}
		targetID, err := requiredActionString(action.RawData, "target_id")
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
			return "", nil, &runtime.Error{
				Code:    "plugin.protocol_violation",
				Message: "onebot action missing target_type",
			}
		}
	case "message.read.mark":
		if messageID, ok := optionalActionString(action.RawData, "message_id"); ok {
			return "mark_msg_as_read", map[string]any{"message_id": oneBotAPIValue(messageID)}, nil
		}
		targetType, err := requiredActionString(action.RawData, "conversation_type")
		if err != nil {
			return "", nil, err
		}
		targetID, err := requiredActionString(action.RawData, "conversation_id")
		if err != nil {
			return "", nil, err
		}
		switch targetType {
		case "group":
			return "mark_group_msg_as_read", map[string]any{"group_id": oneBotAPIValue(targetID)}, nil
		case "private":
			return "mark_private_msg_as_read", map[string]any{"user_id": oneBotAPIValue(targetID)}, nil
		default:
			return "", nil, &runtime.Error{
				Code:    "plugin.protocol_violation",
				Message: "onebot action missing conversation_type",
			}
		}
	case "friend.request.handle":
		if _, err := requiredActionString(action.RawData, "flag"); err != nil {
			return "", nil, err
		}
		return "set_friend_add_request", params, nil
	case "friend.list":
		return "get_friend_list", nil, nil
	case "friend.remark.set":
		if _, err := requiredActionString(action.RawData, "user_id"); err != nil {
			return "", nil, err
		}
		return "set_friend_remark", params, nil
	case "user.info.get":
		if _, err := requiredActionString(action.RawData, "user_id"); err != nil {
			return "", nil, err
		}
		return "get_stranger_info", params, nil
	case "user.like.send":
		if _, err := requiredActionString(action.RawData, "user_id"); err != nil {
			return "", nil, err
		}
		return "send_like", params, nil
	case "group.list":
		return "get_group_list", nil, nil
	case "group.info.get":
		if _, err := requiredActionString(action.RawData, "group_id"); err != nil {
			return "", nil, err
		}
		return "get_group_info", params, nil
	case "group.member.get":
		if _, err := requiredActionString(action.RawData, "group_id"); err != nil {
			return "", nil, err
		}
		if _, err := requiredActionString(action.RawData, "user_id"); err != nil {
			return "", nil, err
		}
		return "get_group_member_info", params, nil
	case "group.member.list":
		if _, err := requiredActionString(action.RawData, "group_id"); err != nil {
			return "", nil, err
		}
		return "get_group_member_list", params, nil
	case "group.request.handle":
		if _, err := requiredActionString(action.RawData, "flag"); err != nil {
			return "", nil, err
		}
		return "set_group_add_request", params, nil
	case "group.leave":
		if _, err := requiredActionString(action.RawData, "group_id"); err != nil {
			return "", nil, err
		}
		return "set_group_leave", params, nil
	case "group.admin.set":
		return "set_group_admin", params, nil
	case "group.ban.set":
		if whole, ok := action.RawData["whole_group"].(bool); ok && whole {
			delete(params, "user_id")
			delete(params, "duration_seconds")
			delete(params, "duration")
			return "set_group_whole_ban", params, nil
		}
		return "set_group_ban", params, nil
	case "group.card.set":
		return "set_group_card", params, nil
	case "group.title.set":
		return "set_group_special_title", params, nil
	case "group.name.set":
		return "set_group_name", params, nil
	case "group.announcement.list":
		return "_get_group_notice", params, nil
	case "group.announcement.create":
		return "_send_group_notice", params, nil
	case "group.announcement.delete":
		return "_del_group_notice", params, nil
	case "group.essence.list":
		return "get_essence_msg_list", params, nil
	case "group.essence.set":
		return "set_essence_msg", params, nil
	case "group.essence.unset":
		return "delete_essence_msg", params, nil
	case "group.honor.get":
		return "get_group_honor_info", params, nil
	case "group.todo.set":
		return "set_group_todo", params, nil
	case "file.get":
		return "get_file", params, nil
	case "file.download":
		return "download_file", params, nil
	case "file.group.upload":
		return "upload_group_file", params, nil
	case "file.private.upload":
		return "upload_private_file", params, nil
	case "file.group.url.get":
		return "get_group_file_url", params, nil
	case "file.private.url.get":
		return "get_private_file_url", params, nil
	case "file.group.fs.info":
		return "get_group_file_system_info", params, nil
	case "file.group.fs.list":
		if folderID, ok := optionalActionString(action.RawData, "folder_id"); ok {
			params["folder_id"] = folderID
			return "get_group_files_by_folder", params, nil
		}
		return "get_group_root_files", params, nil
	case "file.group.fs.mkdir":
		return "create_group_file_folder", params, nil
	case "file.group.fs.delete":
		if folderID, ok := optionalActionString(action.RawData, "folder_id"); ok && folderID != "" {
			return "delete_group_folder", map[string]any{
				"group_id":  params["group_id"],
				"folder_id": folderID,
			}, nil
		}
		return "delete_group_file", params, nil
	case "reaction.set":
		return "set_msg_emoji_like", params, nil
	case "reaction.list":
		return "get_msg_emoji_like_list", params, nil
	case "poke.send":
		return "send_poke", params, nil
	default:
		return "", nil, &runtime.Error{
			Code:    "adapter.compatibility_not_supported",
			Message: "当前 OneBot 动作已冻结但尚未实现",
		}
	}
}

func normalizeActionParams(raw map[string]any) (map[string]any, error) {
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
			params[normalizedKey] = oneBotAPIValue(fmt.Sprint(value))
		default:
			params[normalizedKey] = value
		}
	}
	return params, nil
}

func projectOneBotActionResult(kind string, result any) map[string]any {
	switch typed := result.(type) {
	case nil:
		return map[string]any{"ok": true}
	case map[string]any:
		if len(typed) == 0 {
			return map[string]any{"ok": true}
		}
		return typed
	case []any:
		return map[string]any{projectOneBotCollectionKey(kind): typed}
	default:
		return map[string]any{"value": typed}
	}
}

func projectOneBotCollectionKey(kind string) string {
	switch kind {
	case "friend.list":
		return "friends"
	case "group.list":
		return "groups"
	case "group.member.list":
		return "members"
	case "message.history.get", "message.forward.get", "group.essence.list":
		return "messages"
	case "group.announcement.list":
		return "announcements"
	case "reaction.list":
		return "reactions"
	case "provider.luckylillia.friend_groups.get":
		return "groups"
	default:
		return "items"
	}
}
