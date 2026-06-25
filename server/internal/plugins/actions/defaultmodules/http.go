package defaultmodules

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/httpaction"
)

func init() {
	register(Metadata{
		Action:          "http.request",
		Capability:      "http.request",
		RequestSchema:   "plugin-protocol.action_http_request",
		ResponseSchema:  "plugin-protocol.local_action_result",
		AccessesNetwork: true,
		AuditFields:     []string{"plugin_id", "method", "url"},
		ErrorCodes:      commonErrorCodes("platform.invalid_request"),
	}, func(deps actions.Deps) actions.ActionHandler {
		return func(ctx context.Context, req actions.ActionRequest) (map[string]any, error) {
			return httpaction.Execute(ctx, httpaction.Request{
				PluginID:           req.PluginID,
				Action:             req.Action,
				Config:             currentConfig(deps),
				Capabilities:       deps.Capabilities,
				CredentialInjector: deps.HTTPCredentials,
			})
		}
	})
}
