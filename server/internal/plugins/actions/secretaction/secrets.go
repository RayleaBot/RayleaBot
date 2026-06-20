package secretaction

import (
	"context"
	"regexp"
	"strings"

	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

var pluginSecretKeyPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_.-]{0,126}[a-z0-9])?$`)

type CapabilityView interface {
	CapabilityDeclared(context.Context, string, string) bool
}

type Reader interface {
	ReadPluginSecret(context.Context, string) (string, bool, error)
}

type Request struct {
	PluginID     string
	Action       runtimeaction.Action
	Capabilities CapabilityView
	Reader       Reader
}

func ExecuteRead(ctx context.Context, req Request) (map[string]any, error) {
	if req.Capabilities == nil || !req.Capabilities.CapabilityDeclared(ctx, req.PluginID, "secret.read") {
		return nil, &runtimemanager.Error{
			Code:    "plugin.capability_violation",
			Message: "secret.read capability is not declared",
		}
	}

	key := strings.TrimSpace(req.Action.SecretKey)
	if !isPluginSecretKey(key) {
		return nil, &runtimemanager.Error{
			Code:    "plugin.protocol_violation",
			Message: "secret.read key is required",
		}
	}
	if req.Reader == nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "secret.read store is not available",
		}
	}

	value, exists, err := req.Reader.ReadPluginSecret(ctx, pluginSecretStorageKey(req.PluginID, key))
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
