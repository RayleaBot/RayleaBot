package app

import (
	"context"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (c *pluginLifecycleController) buildStartInputs(ctx context.Context, pluginID, botID string) (runtime.Spec, runtime.InitPayload, error) {
	return c.buildStartInputsWithCapabilities(pluginID, botID, c.grants.grantedCapabilities(ctx, pluginID))
}

func (c *pluginLifecycleController) buildStartInputsWithCapabilities(pluginID, botID string, capabilities []string) (runtime.Spec, runtime.InitPayload, error) {
	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return runtime.Spec{}, runtime.InitPayload{}, plugins.ErrPluginNotFound
	}

	spec, err := runtime.BuildSpec(snapshot, c.state.repoRoot, c.state.Config.Runtime)
	if err != nil {
		return runtime.Spec{}, runtime.InitPayload{}, err
	}

	payload := runtime.InitPayload{
		Bot: runtime.BotInfo{
			ID: strings.TrimSpace(botID),
		},
		Capabilities:    append([]string(nil), capabilities...),
		SuperAdmins:     pluginRuntimeSuperAdmins(c.state.Config),
		CommandPrefixes: runtimeCommandPrefixes(c.state.Config),
	}
	return spec, payload, nil
}

func pluginRuntimeSuperAdmins(cfg config.Config) []string {
	source := cfg.Admin.SuperAdmins
	result := make([]string, 0, len(source))
	seen := make(map[string]struct{}, len(source))
	for _, item := range source {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
