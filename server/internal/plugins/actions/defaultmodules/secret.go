package defaultmodules

import (
	"context"
	"regexp"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

var pluginSecretKeyPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_.-]{0,126}[a-z0-9])?$`)

func init() {
	register(Metadata{
		Action:         "secret.read",
		Capability:     "secret.read",
		RequestSchema:  "plugin-protocol.action_secret_read",
		ResponseSchema: "plugin-protocol.local_action_result",
		ReadsSecret:    true,
		AuditFields:    []string{"plugin_id", "key", "exists"},
		ErrorCodes:     commonErrorCodes(),
	}, func(deps actions.Deps) actions.ActionHandler {
		return func(ctx context.Context, req actions.ActionRequest) (map[string]any, error) {
			return executeSecretRead(ctx, deps, req)
		}
	})
}

func executeSecretRead(ctx context.Context, deps actions.Deps, req actions.ActionRequest) (map[string]any, error) {
	if deps.Capabilities == nil || !deps.Capabilities.CapabilityDeclared(ctx, req.PluginID, "secret.read") {
		return nil, &runtimemanager.Error{Code: "plugin.capability_violation", Message: "secret.read capability is not declared"}
	}

	key := strings.TrimSpace(req.Action.SecretKey)
	if !isPluginSecretKey(key) {
		return nil, &runtimemanager.Error{Code: "plugin.protocol_violation", Message: "secret.read key is required"}
	}
	if deps.Secrets == nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "secret.read store is not available"}
	}

	value, exists, err := deps.Secrets.ReadPluginSecret(ctx, pluginSecretStorageKey(req.PluginID, key))
	if err != nil {
		return nil, &runtimemanager.Error{Code: "plugin.internal_error", Message: "secret.read failed", Err: err}
	}
	if !exists {
		return map[string]any{"key": key, "exists": false}, nil
	}
	return map[string]any{"key": key, "exists": true, "value": value}, nil
}

func pluginSecretStorageKey(pluginID, key string) string {
	return "plugin:" + strings.TrimSpace(pluginID) + ":secret:" + strings.TrimSpace(key)
}

func isPluginSecretKey(key string) bool {
	return pluginSecretKeyPattern.MatchString(strings.TrimSpace(key))
}
