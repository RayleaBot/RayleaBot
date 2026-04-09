package app

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
	return strings.HasPrefix(kind, "provider.napcat.") || strings.HasPrefix(kind, "provider.luckylillia.")
}

func (a *App) executeOneBotLocalAction(ctx context.Context, _ string, _ string, action runtime.Action) (map[string]any, error) {
	if runtimeIsProviderExtensionAction(action.Kind) {
		return nil, &runtime.Error{
			Code:    "adapter.provider_extension_not_supported",
			Message: "当前 provider 扩展动作尚未实现",
		}
	}
	if a.Adapter == nil {
		return nil, &runtime.Error{
			Code:    "adapter.transport_not_implemented",
			Message: "OneBot adapter 不可用",
		}
	}

	switch action.Kind {
	case "user.info.get":
		userID, err := requiredActionString(action.RawData, "user_id")
		if err != nil {
			return nil, err
		}
		info, callErr := a.Adapter.GetStrangerInfo(ctx, userID)
		if callErr != nil {
			return nil, toRuntimeActionError(callErr)
		}
		return map[string]any{
			"user_id":  userID,
			"nickname": info.Nickname,
		}, nil
	case "group.info.get":
		groupID, err := requiredActionString(action.RawData, "group_id")
		if err != nil {
			return nil, err
		}
		info, callErr := a.Adapter.GetGroupInfo(ctx, groupID)
		if callErr != nil {
			return nil, toRuntimeActionError(callErr)
		}
		return map[string]any{
			"group_id":   groupID,
			"group_name": info.Name,
		}, nil
	case "group.member.get":
		groupID, err := requiredActionString(action.RawData, "group_id")
		if err != nil {
			return nil, err
		}
		userID, err := requiredActionString(action.RawData, "user_id")
		if err != nil {
			return nil, err
		}
		info, callErr := a.Adapter.GetGroupMemberInfo(ctx, groupID, userID)
		if callErr != nil {
			return nil, toRuntimeActionError(callErr)
		}
		return map[string]any{
			"group_id": groupID,
			"user_id":  userID,
			"role":     info.Role,
			"nickname": info.Nickname,
			"card":     info.Card,
		}, nil
	default:
		return nil, &runtime.Error{
			Code:    "adapter.compatibility_not_supported",
			Message: "当前 OneBot 动作已冻结但尚未实现",
		}
	}
}

func requiredActionString(data map[string]any, key string) (string, error) {
	if len(data) == 0 {
		return "", &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: fmt.Sprintf("onebot action missing %s", key),
		}
	}
	value, ok := data[key]
	if !ok {
		return "", &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: fmt.Sprintf("onebot action missing %s", key),
		}
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return "", &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: fmt.Sprintf("onebot action missing %s", key),
		}
	}
	return text, nil
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
