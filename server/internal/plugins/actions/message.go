package actions

import (
	"context"

	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func messageSendRegistrar() registrar {
	return registrar{
		kind: "message.send",
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeMessageSend(ctx, deps, req)
			}
		},
	}
}

func executeMessageSend(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Permissions == nil || !deps.Permissions.PermissionDeclared(ctx, req.PluginID, "message.send") {
		return nil, &pluginruntime.Error{
			Code:    "plugin.permission_denied",
			Message: "message.send permission is not declared",
		}
	}
	if deps.MessageSender == nil {
		return nil, &pluginruntime.Error{
			Code:    "plugin.internal_error",
			Message: "message.send outbound sender is not available",
		}
	}
	return deps.MessageSender(ctx, req.PluginID, req.RequestID, req.ParentEvent, req.Action)
}
