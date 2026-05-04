package localaction

import (
	"context"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (s *Service) executeConfigRead(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "config.read") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "config.read capability is not granted",
		}
	}
	if s.pluginConfig == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "config.read repository is not available",
		}
	}

	values, err := s.pluginConfig.Read(ctx, pluginID, action.ConfigKeys)
	if err != nil {
		return nil, &runtime.Error{Code: "plugin.internal_error", Message: "config.read failed", Err: err}
	}
	return map[string]any{
		"values": values,
	}, nil
}

func (s *Service) executeConfigWrite(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "config.write") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "config.write capability is not granted",
		}
	}
	if s.pluginConfig == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "config.write repository is not available",
		}
	}

	changedKeys, err := s.pluginConfig.Write(ctx, pluginID, action.ConfigValues)
	if err != nil {
		return nil, &runtime.Error{Code: "plugin.internal_error", Message: "config.write failed", Err: err}
	}
	if len(changedKeys) > 0 && s.refreshCommands != nil {
		if settings, readErr := s.pluginConfig.ReadAll(ctx, pluginID); readErr != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "config.write failed", Err: readErr}
		} else {
			s.refreshCommands(ctx, pluginID, settings)
		}
	}
	s.dispatchPluginConfigChanged(ctx, pluginID)
	return map[string]any{
		"changed_keys": changedKeys,
	}, nil
}

func (s *Service) executeLoggerWrite(ctx context.Context, pluginID, requestID string, action runtime.Action) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "logger.write") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "logger.write capability is not granted",
		}
	}
	if s.pluginLogLimiter != nil && !s.pluginLogLimiter.Allow(pluginID) {
		return nil, &runtime.Error{
			Code:    "platform.rate_limited",
			Message: "plugin log throughput exceeded the configured platform limit",
		}
	}
	if s.logger == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "logger.write is not available",
		}
	}

	level := strings.TrimSpace(action.LogLevel)
	message := action.LogMessage
	if s.redactText != nil {
		message = s.redactText(message)
	}
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
		attrs = append(attrs, key, redactValue(s.redactText, action.LogFields[key]))
	}

	switch level {
	case "debug":
		s.logger.Debug(message, attrs...)
	case "warn":
		s.logger.Warn(message, attrs...)
	case "error":
		s.logger.Error(message, attrs...)
	default:
		s.logger.Info(message, attrs...)
	}
	return map[string]any{}, nil
}
