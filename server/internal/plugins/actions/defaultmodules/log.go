package defaultmodules

import (
	"context"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

func init() {
	register(Metadata{
		Action:         "logger.write",
		Capability:     "logger.write",
		RequestSchema:  "plugin-protocol.action_logger_write",
		ResponseSchema: "plugin-protocol.local_action_result",
		AuditFields:    []string{"plugin_id", "request_id", "level"},
		ErrorCodes:     commonErrorCodes("platform.rate_limited"),
	}, func(deps actions.Deps) actions.ActionHandler {
		return func(ctx context.Context, req actions.ActionRequest) (map[string]any, error) {
			return executeLogWrite(ctx, deps, req)
		}
	})
}

func executeLogWrite(ctx context.Context, deps actions.Deps, req actions.ActionRequest) (map[string]any, error) {
	if deps.Capabilities == nil || !deps.Capabilities.CapabilityDeclared(ctx, req.PluginID, "logger.write") {
		return nil, &runtimemanager.Error{Code: "plugin.capability_violation", Message: "logger.write capability is not declared"}
	}
	if deps.PluginLogLimiter != nil && !deps.PluginLogLimiter.Allow(req.PluginID) {
		return nil, &runtimemanager.Error{Code: "platform.rate_limited", Message: "plugin log throughput exceeded the configured platform limit"}
	}
	if deps.Logger == nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "logger.write is not available"}
	}

	level := strings.TrimSpace(req.Action.LogLevel)
	message := req.Action.LogMessage
	if deps.RedactText != nil {
		message = deps.RedactText(message)
	}
	attrs := []any{
		"component", "plugin",
		"plugin_id", req.PluginID,
		"request_id", req.RequestID,
	}
	keys := make([]string, 0, len(req.Action.LogFields))
	for key := range req.Action.LogFields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		attrs = append(attrs, key, redactLogValue(deps.RedactText, req.Action.LogFields[key]))
	}

	switch level {
	case "debug":
		deps.Logger.Debug(message, attrs...)
	case "warn":
		deps.Logger.Warn(message, attrs...)
	case "error":
		deps.Logger.Error(message, attrs...)
	default:
		deps.Logger.Info(message, attrs...)
	}
	return map[string]any{}, nil
}

func redactLogValue(redactText func(string) string, value any) any {
	switch typed := value.(type) {
	case string:
		if redactText == nil {
			return typed
		}
		return redactText(typed)
	case []any:
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = redactLogValue(redactText, typed[index])
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, inner := range typed {
			result[key] = redactLogValue(redactText, inner)
		}
		return result
	default:
		return value
	}
}
