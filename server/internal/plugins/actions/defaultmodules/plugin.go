package defaultmodules

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/pluginlist"
)

func init() {
	register(Metadata{
		Action:             "plugin.list",
		Capability:         "plugin.list",
		RequestSchema:      "plugin-protocol.action_plugin_list",
		ResponseSchema:     "plugin-protocol.local_action_result",
		RequiredPermission: "declared capability",
		AuditFields:        []string{"plugin_id", "visibility"},
		ErrorCodes:         commonErrorCodes(),
	}, func(deps actions.Deps) actions.ActionHandler {
		return func(ctx context.Context, req actions.ActionRequest) (map[string]any, error) {
			return pluginlist.Execute(ctx, pluginlist.Request{
				PluginID:      req.PluginID,
				Action:        req.Action,
				ParentEvent:   req.ParentEvent,
				Capabilities:  deps.Capabilities,
				CurrentConfig: deps.CurrentConfig,
			})
		}
	})
}
