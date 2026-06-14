package service

import (
	"context"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimespec "github.com/RayleaBot/RayleaBot/server/internal/runtime/spec"
)

func (c *Controller) buildStartInputs(ctx context.Context, pluginID, botID string) (runtimespec.Spec, runtimespec.InitPayload, error) {
	return c.buildStartInputsWithCapabilities(pluginID, botID, c.grants.grantedCapabilities(ctx, pluginID))
}

func (c *Controller) buildStartInputsWithCapabilities(pluginID, botID string, capabilities []string) (runtimespec.Spec, runtimespec.InitPayload, error) {
	snapshot, ok := c.plugins.Get(pluginID)
	if !ok {
		return runtimespec.Spec{}, runtimespec.InitPayload{}, plugins.ErrPluginNotFound
	}

	cfg := c.config()
	spec, err := runtimespec.BuildSpec(snapshot, c.repoRoot, cfg.Runtime)
	if err != nil {
		return runtimespec.Spec{}, runtimespec.InitPayload{}, err
	}

	payload := runtimespec.InitPayload{
		Bot: runtimespec.BotInfo{
			ID: strings.TrimSpace(botID),
		},
		Capabilities:    append([]string(nil), capabilities...),
		SuperAdmins:     pluginRuntimeSuperAdmins(cfg),
		CommandPrefixes: runtimeCommandPrefixes(cfg),
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

func PluginRuntimeSuperAdmins(cfg config.Config) []string {
	return pluginRuntimeSuperAdmins(cfg)
}
