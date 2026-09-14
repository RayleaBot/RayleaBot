package actions

import (
	"context"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func logWriteRegistrar() registrar {
	return registrar{
		kind: "logger.write",
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeLogWrite(deps, req)
			}
		},
	}
}

func executeLogWrite(deps Deps, req ActionRequest) (map[string]any, error) {
	if deps.Logger == nil {
		return nil, &plugins.Error{Code: errorcodes.PluginInternalError, Message: "logger.write is not available"}
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
