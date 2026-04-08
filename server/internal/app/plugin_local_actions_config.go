package app

import (
	"context"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (a *App) executeConfigRead(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if !a.pluginCapabilityGranted(ctx, pluginID, "config.read") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "config.read capability is not granted",
		}
	}
	if a == nil || a.pluginConfig == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "config.read repository is not available",
		}
	}

	values, err := a.pluginConfig.Read(ctx, pluginID, action.ConfigKeys)
	if err != nil {
		return nil, &runtime.Error{Code: "plugin.internal_error", Message: "config.read failed", Err: err}
	}
	return map[string]any{
		"values": values,
	}, nil
}

func (a *App) executeConfigWrite(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if !a.pluginCapabilityGranted(ctx, pluginID, "config.write") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "config.write capability is not granted",
		}
	}
	if a == nil || a.pluginConfig == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "config.write repository is not available",
		}
	}

	changedKeys, err := a.pluginConfig.Write(ctx, pluginID, action.ConfigValues)
	if err != nil {
		return nil, &runtime.Error{Code: "plugin.internal_error", Message: "config.write failed", Err: err}
	}
	a.dispatchPluginConfigChanged(ctx, pluginID)
	return map[string]any{
		"changed_keys": changedKeys,
	}, nil
}

func (a *App) executeLoggerWrite(ctx context.Context, pluginID, requestID string, action runtime.Action) (map[string]any, error) {
	if !a.pluginCapabilityGranted(ctx, pluginID, "logger.write") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "logger.write capability is not granted",
		}
	}
	if a.pluginLogLimiter != nil && !a.pluginLogLimiter.Allow(pluginID) {
		return nil, &runtime.Error{
			Code:    "platform.rate_limited",
			Message: "plugin log throughput exceeded the configured platform limit",
		}
	}
	if a == nil || a.Logger == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "logger.write is not available",
		}
	}

	level := strings.TrimSpace(action.LogLevel)
	message := a.redactString(action.LogMessage)
	attrs := []any{
		"component", "plugin",
		"plugin_id", pluginID,
		"request_id", requestID,
	}
	keys := make([]string, 0, len(action.LogFields))
	for key := range action.LogFields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		attrs = append(attrs, key, redactValue(a.redactText, action.LogFields[key]))
	}

	switch level {
	case "debug":
		a.Logger.Debug(message, attrs...)
	case "warn":
		a.Logger.Warn(message, attrs...)
	case "error":
		a.Logger.Error(message, attrs...)
	default:
		a.Logger.Info(message, attrs...)
	}
	return map[string]any{}, nil
}
