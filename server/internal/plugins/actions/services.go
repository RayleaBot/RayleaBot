package actions

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type ServiceCallFunc func(context.Context, string, plugins.ServiceCall, chatevent.Event) (map[string]any, error)

func pluginCallRegistrar() registrar {
	return registrar{kind: "plugin.call", factory: func(deps Deps) ActionHandler {
		return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
			if req.Action.ServiceCall == nil {
				return nil, &plugins.Error{Code: errorcodes.PluginProtocolViolation, Message: "service call arguments are required"}
			}
			if deps.CallService == nil {
				return nil, &plugins.Error{Code: errorcodes.PluginServiceUnavailable, Message: "plugin service routing is unavailable"}
			}
			return deps.CallService(ctx, req.PluginID, *req.Action.ServiceCall, req.ParentEvent)
		}
	}}
}
