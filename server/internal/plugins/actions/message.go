package actions

import (
	"context"

	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func messageSendRegistrar() registrar {
	return registrar{
		metadata: Metadata{
			Action:         "message.send",
			Capability:     "message.send",
			RequestSchema:  "plugin-protocol.action_message_send",
			ResponseSchema: "plugin-protocol.local_action_result",
			AuditFields:    []string{"plugin_id", "target_type", "target_id"},
			ErrorCodes: commonErrorCodes(
				"platform.rate_limited",
				"adapter.send_failed",
			),
		},
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeMessageSend(ctx, deps, req)
			}
		},
	}
}

func executeMessageSend(ctx context.Context, deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Capabilities == nil || !deps.Capabilities.CapabilityDeclared(ctx, req.PluginID, "message.send") {
		return nil, &pluginruntime.Error{
			Code:    "plugin.capability_violation",
			Message: "message.send capability is not declared",
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
