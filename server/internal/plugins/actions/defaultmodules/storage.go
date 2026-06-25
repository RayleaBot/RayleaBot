package defaultmodules

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/storageaction"
)

func init() {
	register(Metadata{
		Action:         "storage.kv",
		Capability:     "storage.kv",
		RequestSchema:  "plugin-protocol.action_storage_kv",
		ResponseSchema: "plugin-protocol.local_action_result",
		AuditFields:    []string{"plugin_id", "operation", "key", "prefix"},
		ErrorCodes:     commonErrorCodes("platform.value_too_large"),
	}, func(deps actions.Deps) actions.ActionHandler {
		return func(ctx context.Context, req actions.ActionRequest) (map[string]any, error) {
			return storageaction.ExecuteKV(ctx, storageaction.Request{
				PluginID:     req.PluginID,
				Action:       req.Action,
				Config:       currentConfig(deps),
				Capabilities: deps.Capabilities,
				KV:           deps.PluginKV,
			})
		}
	})
	register(Metadata{
		Action:         "storage.file",
		Capability:     "storage.file",
		RequestSchema:  "plugin-protocol.action_storage_file",
		ResponseSchema: "plugin-protocol.local_action_result",
		WritesFile:     true,
		AuditFields:    []string{"plugin_id", "operation", "root", "path", "prefix"},
		ErrorCodes:     commonErrorCodes("platform.invalid_request", "platform.value_too_large"),
	}, func(deps actions.Deps) actions.ActionHandler {
		return func(ctx context.Context, req actions.ActionRequest) (map[string]any, error) {
			return storageaction.ExecuteFile(ctx, storageaction.Request{
				PluginID:     req.PluginID,
				Action:       req.Action,
				Config:       currentConfig(deps),
				Capabilities: deps.Capabilities,
				Files:        deps.PluginFiles,
			})
		}
	})
}
