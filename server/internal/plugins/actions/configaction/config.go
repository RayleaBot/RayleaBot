package configaction

import (
	"context"
	"log/slog"

	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

type Grants interface {
	CapabilityGranted(context.Context, string, string) bool
}

type Repository interface {
	Read(context.Context, string, []string) (map[string]any, error)
	ReadAll(context.Context, string) (map[string]any, error)
	Write(context.Context, string, map[string]any) ([]string, error)
}

type DispatchResult struct {
	Delivered bool
	Outcome   string
	ErrorCode string
}

type Dispatcher func(context.Context, string) DispatchResult

type CommandRefresher func(context.Context, string, map[string]any)

type Request struct {
	PluginID        string
	Action          runtimeaction.Action
	Grants          Grants
	Repository      Repository
	RefreshCommands CommandRefresher
	Dispatcher      Dispatcher
	Logger          *slog.Logger
}

func ExecuteRead(ctx context.Context, req Request) (map[string]any, error) {
	if req.Grants == nil || !req.Grants.CapabilityGranted(ctx, req.PluginID, "config.read") {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: "config.read capability is not granted",
		}
	}
	if req.Repository == nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "config.read repository is not available",
		}
	}

	values, err := req.Repository.Read(ctx, req.PluginID, req.Action.ConfigKeys)
	if err != nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "config.read failed", Err: err}
	}
	return map[string]any{
		"values": values,
	}, nil
}

func ExecuteWrite(ctx context.Context, req Request) (map[string]any, error) {
	if req.Grants == nil || !req.Grants.CapabilityGranted(ctx, req.PluginID, "config.write") {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: "config.write capability is not granted",
		}
	}
	if req.Repository == nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "config.write repository is not available",
		}
	}

	changedKeys, err := req.Repository.Write(ctx, req.PluginID, req.Action.ConfigValues)
	if err != nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "config.write failed", Err: err}
	}
	if len(changedKeys) > 0 && req.RefreshCommands != nil {
		settings, readErr := req.Repository.ReadAll(ctx, req.PluginID)
		if readErr != nil {
			return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "config.write failed", Err: readErr}
		}
		req.RefreshCommands(ctx, req.PluginID, settings)
	}
	DispatchChanged(ctx, req.PluginID, req.Dispatcher, req.Logger)
	return map[string]any{
		"changed_keys": changedKeys,
	}, nil
}

func DispatchChanged(ctx context.Context, pluginID string, dispatcher Dispatcher, logger *slog.Logger) {
	if dispatcher == nil {
		return
	}

	result := dispatcher(ctx, pluginID)
	if result.Delivered || logger == nil {
		return
	}
	logger.Warn(
		"config.changed event was not queued for plugin runtime",
		"component", "app",
		"plugin_id", pluginID,
		"outcome", result.Outcome,
		"error_code", result.ErrorCode,
	)
}
