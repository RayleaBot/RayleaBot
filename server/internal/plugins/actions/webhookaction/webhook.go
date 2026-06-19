package webhookaction

import (
	"context"

	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

type Gateway interface {
	Expose(context.Context, string, runtimeaction.Action) (map[string]any, error)
}

type Request struct {
	PluginID string
	Action   runtimeaction.Action
	Gateway  Gateway
}

func ExecuteExpose(ctx context.Context, req Request) (map[string]any, error) {
	if req.Gateway == nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "webhook gateway is not available",
		}
	}
	return req.Gateway.Expose(ctx, req.PluginID, req.Action)
}
