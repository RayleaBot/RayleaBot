package actions

import (
	"context"
	"regexp"
	"strings"

	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

var pluginSecretKeyPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_.-]{0,126}[a-z0-9])?$`)

func (s *Service) executeSecretRead(ctx context.Context, pluginID string, action runtimeaction.Action) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "secret.read") {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: "secret.read capability is not granted",
		}
	}

	key := strings.TrimSpace(action.SecretKey)
	if !isPluginSecretKey(key) {
		return nil, &runtimemanager.Error{
			Code:    "plugin.protocol_violation",
			Message: "secret.read key is required",
		}
	}
	if s.secrets == nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "secret.read store is not available",
		}
	}

	value, exists, err := s.secrets.ReadPluginSecret(ctx, pluginSecretStorageKey(pluginID, key))
	if err != nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "secret.read failed", Err: err}
	}
	if !exists {
		return map[string]any{
			"key":    key,
			"exists": false,
		}, nil
	}

	return map[string]any{
		"key":    key,
		"exists": true,
		"value":  value,
	}, nil
}

func pluginSecretStorageKey(pluginID, key string) string {
	return "plugin:" + strings.TrimSpace(pluginID) + ":secret:" + strings.TrimSpace(key)
}

func isPluginSecretKey(key string) bool {
	return pluginSecretKeyPattern.MatchString(strings.TrimSpace(key))
}
