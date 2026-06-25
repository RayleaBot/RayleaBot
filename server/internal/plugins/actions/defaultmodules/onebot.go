package defaultmodules

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	localonebot "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/onebot"
)

func init() {
	for kind, spec := range localonebot.Registry() {
		kind := kind
		spec := spec
		register(Metadata{
			Action:         kind,
			Capability:     spec.Capability,
			RequestSchema:  "plugin-protocol.onebot_action",
			ResponseSchema: "plugin-protocol.local_action_result",
			AuditFields:    []string{"plugin_id", "action", "provider"},
			ErrorCodes:     commonErrorCodes("adapter.transport_not_implemented", "adapter.provider_extension_not_supported"),
		}, func(deps actions.Deps) actions.ActionHandler {
			return func(ctx context.Context, req actions.ActionRequest) (map[string]any, error) {
				return localonebot.Execute(ctx, localonebot.Request{
					PluginID:     req.PluginID,
					Action:       req.Action,
					Capabilities: deps.Capabilities,
					Adapter:      deps.Adapter,
				})
			}
		})
	}
}
