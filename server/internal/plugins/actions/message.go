package actions

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
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
		return nil, &plugins.Error{
			Code:    errorcodes.PluginPermissionDenied,
			Message: "message.send permission is not declared",
		}
	}
	if deps.MessageSender == nil {
		return nil, &plugins.Error{
			Code:    errorcodes.PluginInternalError,
			Message: "message.send outbound sender is not available",
		}
	}
	return deps.MessageSender(ctx, req.PluginID, req.RequestID, req.ParentEvent, req.Action.MessageCommand())
}
