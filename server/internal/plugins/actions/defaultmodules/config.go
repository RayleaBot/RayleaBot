package defaultmodules

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

func init() {
	register(Metadata{
		Action:         "config.read",
		Capability:     "config.read",
		RequestSchema:  "plugin-protocol.action_config_read",
		ResponseSchema: "plugin-protocol.local_action_result",
		AuditFields:    []string{"plugin_id", "keys"},
		ErrorCodes:     commonErrorCodes(),
	}, func(deps actions.Deps) actions.ActionHandler {
		return func(ctx context.Context, req actions.ActionRequest) (map[string]any, error) {
			return executeConfigRead(ctx, deps, req)
		}
	})
	register(Metadata{
		Action:         "config.write",
		Capability:     "config.write",
		RequestSchema:  "plugin-protocol.action_config_write",
		ResponseSchema: "plugin-protocol.local_action_result",
		AuditFields:    []string{"plugin_id", "changed_keys"},
		ErrorCodes:     commonErrorCodes(),
	}, func(deps actions.Deps) actions.ActionHandler {
		return func(ctx context.Context, req actions.ActionRequest) (map[string]any, error) {
			return executeConfigWrite(ctx, deps, req)
		}
	})
}

func executeConfigRead(ctx context.Context, deps actions.Deps, req actions.ActionRequest) (map[string]any, error) {
	if deps.Capabilities == nil || !deps.Capabilities.CapabilityDeclared(ctx, req.PluginID, "config.read") {
		return nil, &runtimemanager.Error{Code: "plugin.capability_violation", Message: "config.read capability is not declared"}
	}
	if deps.PluginConfig == nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "config.read repository is not available"}
	}
	values, err := deps.PluginConfig.Read(ctx, req.PluginID, req.Action.ConfigKeys)
	if err != nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "config.read failed", Err: err}
	}
	return map[string]any{"values": values}, nil
}

func executeConfigWrite(ctx context.Context, deps actions.Deps, req actions.ActionRequest) (map[string]any, error) {
	if deps.Capabilities == nil || !deps.Capabilities.CapabilityDeclared(ctx, req.PluginID, "config.write") {
		return nil, &runtimemanager.Error{Code: "plugin.capability_violation", Message: "config.write capability is not declared"}
	}
	if deps.PluginConfig == nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "config.write repository is not available"}
	}

	changedKeys, err := deps.PluginConfig.Write(ctx, req.PluginID, req.Action.ConfigValues)
	if err != nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "config.write failed", Err: err}
	}
	if len(changedKeys) > 0 && deps.RefreshCommands != nil {
		settings, readErr := deps.PluginConfig.ReadAll(ctx, req.PluginID)
		if readErr != nil {
			return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "config.write failed", Err: readErr}
		}
		deps.RefreshCommands(ctx, req.PluginID, settings)
	}
	dispatchConfigChanged(ctx, req.PluginID, deps.Dispatcher, deps.Logger)
	return map[string]any{"changed_keys": changedKeys}, nil
}

func dispatchConfigChanged(ctx context.Context, pluginID string, dispatcher actions.ConfigChangeDispatcher, logger interface {
	Warn(string, ...any)
}) {
	if dispatcher == nil {
		return
	}
	result := dispatcher(ctx, pluginID)
	if result.Delivered || logger == nil {
		return
	}
	logger.Warn(
		"插件 "+pluginID+" 配置变更事件未能投递到运行时",
		"component", "app",
		"plugin_id", pluginID,
		"outcome", result.Outcome,
		"error_code", result.ErrorCode,
	)
}
