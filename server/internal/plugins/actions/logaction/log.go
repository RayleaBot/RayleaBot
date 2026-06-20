package logaction

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

type CapabilityView interface {
	CapabilityDeclared(context.Context, string, string) bool
}

type Limiter interface {
	Allow(string) bool
}

type Request struct {
	PluginID     string
	RequestID    string
	Action       runtimeaction.Action
	Capabilities CapabilityView
	Logger       *slog.Logger
	RedactText   func(string) string
	Limiter      Limiter
}

func Execute(ctx context.Context, req Request) (map[string]any, error) {
	if req.Capabilities == nil || !req.Capabilities.CapabilityDeclared(ctx, req.PluginID, "logger.write") {
		return nil, &runtimemanager.Error{
			Code:    "plugin.capability_violation",
			Message: "logger.write capability is not declared",
		}
	}
	if req.Limiter != nil && !req.Limiter.Allow(req.PluginID) {
		return nil, &runtimemanager.Error{
			Code:    "platform.rate_limited",
			Message: "plugin log throughput exceeded the configured platform limit",
		}
	}
	if req.Logger == nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "logger.write is not available",
		}
	}

	level := strings.TrimSpace(req.Action.LogLevel)
	message := req.Action.LogMessage
	if req.RedactText != nil {
		message = req.RedactText(message)
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
		attrs = append(attrs, key, redactValue(req.RedactText, req.Action.LogFields[key]))
	}

	switch level {
	case "debug":
		req.Logger.Debug(message, attrs...)
	case "warn":
		req.Logger.Warn(message, attrs...)
	case "error":
		req.Logger.Error(message, attrs...)
	default:
		req.Logger.Info(message, attrs...)
	}
	return map[string]any{}, nil
}

func redactValue(redactText func(string) string, value any) any {
	switch typed := value.(type) {
	case string:
		if redactText == nil {
			return typed
		}
		return redactText(typed)
	case []any:
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = redactValue(redactText, typed[index])
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, inner := range typed {
			result[key] = redactValue(redactText, inner)
		}
		return result
	default:
		return value
	}
}
