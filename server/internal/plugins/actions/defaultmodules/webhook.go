package defaultmodules

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

func init() {
	register(Metadata{
		Action:          "event.expose_webhook",
		Capability:      "event.expose_webhook",
		RequestSchema:   "plugin-protocol.action_event_expose_webhook",
		ResponseSchema:  "plugin-protocol.local_action_result",
		AccessesNetwork: true,
		AuditFields:     []string{"plugin_id", "route_id"},
		ErrorCodes:      commonErrorCodes("platform.invalid_request"),
	}, func(deps actions.Deps) actions.ActionHandler {
		return func(ctx context.Context, req actions.ActionRequest) (map[string]any, error) {
			return executeWebhookExpose(ctx, deps, req)
		}
	})
}

func executeWebhookExpose(ctx context.Context, deps actions.Deps, req actions.ActionRequest) (map[string]any, error) {
	if deps.WebhookGateway == nil || deps.WebhookGateway() == nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "webhook gateway is not available"}
	}
	return deps.WebhookGateway().Expose(ctx, req.PluginID, req.Action)
}
