package localaction

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

var pluginSecretKeyPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_.-]{0,126}[a-z0-9])?$`)

func (s *Service) executeSecretRead(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "secret.read") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "secret.read capability is not granted",
		}
	}

	key := strings.TrimSpace(action.SecretKey)
	if !isPluginSecretKey(key) {
		return nil, &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: "secret.read key is required",
		}
	}
	if s.secrets == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "secret.read store is not available",
		}
	}

	value, err := s.secrets.Get(ctx, pluginSecretStorageKey(pluginID, key))
	if err != nil {
		if errors.Is(err, secrets.ErrNotFound) {
			return map[string]any{
				"key":    key,
				"exists": false,
			}, nil
		}
		return nil, &runtime.Error{Code: "plugin.internal_error", Message: "secret.read failed", Err: err}
	}

	plaintext, err := secrets.OpenString(ctx, s.secrets, value)
	if err != nil {
		return nil, &runtime.Error{Code: "plugin.internal_error", Message: "secret.read failed", Err: err}
	}

	return map[string]any{
		"key":    key,
		"exists": true,
		"value":  plaintext,
	}, nil
}

func pluginSecretStorageKey(pluginID, key string) string {
	return "plugin:" + strings.TrimSpace(pluginID) + ":secret:" + strings.TrimSpace(key)
}

func isPluginSecretKey(key string) bool {
	return pluginSecretKeyPattern.MatchString(strings.TrimSpace(key))
}
