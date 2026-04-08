package app

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func (a *App) persistPluginDesiredState(ctx context.Context, pluginID, desiredState string) error {
	if a == nil || a.pluginRepository == nil {
		return nil
	}
	return a.pluginRepository.SaveDesiredState(ctx, pluginID, desiredState, time.Now().UTC())
}

func missingCapabilities(required []string, granted []string) []string {
	if len(required) == 0 {
		return nil
	}

	missing := make([]string, 0, len(required))
	for _, capability := range required {
		if capability == "" || slices.Contains(granted, capability) {
			continue
		}
		missing = append(missing, capability)
	}
	return missing
}

func dispatchCommands(commands []plugins.Command) []dispatch.CommandDecl {
	items := make([]dispatch.CommandDecl, 0, len(commands))
	for _, command := range commands {
		if strings.TrimSpace(command.Name) == "" {
			continue
		}
		items = append(items, dispatch.CommandDecl{
			Name:       command.Name,
			Aliases:    append([]string(nil), command.Aliases...),
			Permission: command.Permission,
		})
	}
	return items
}

func (c *pluginLifecycleController) grantedCapabilities(ctx context.Context, pluginID string) []string {
	auto := append([]string(nil), c.app.Config.Auth.AutoGrantCapabilities...)
	if c.app.grantRepository == nil {
		return auto
	}
	grants, err := c.app.grantRepository.LoadGrants(ctx, pluginID)
	if err != nil {
		return auto
	}
	for _, g := range grants {
		if !slices.Contains(auto, g.Capability) {
			auto = append(auto, g.Capability)
		}
	}
	return auto
}

func runtimeInitTimeout(cfg config.RuntimeConfig) time.Duration {
	seconds := cfg.PluginInitMaxTotalSeconds
	if seconds <= 0 {
		seconds = 300
	}
	return time.Duration(seconds+5) * time.Second
}

func scopeChangedSinceGrant(ctx context.Context, repo plugins.GrantRepository, snapshot plugins.Snapshot) bool {
	grants, err := repo.LoadGrants(ctx, snapshot.PluginID)
	if err != nil || len(grants) == 0 {
		return false
	}
	currentScope := plugins.BuildScopeJSON(snapshot)
	for _, g := range grants {
		if g.ScopeJSON != currentScope {
			return true
		}
	}
	return false
}

func (c *pluginLifecycleController) seedPluginDefaultConfig(ctx context.Context, snapshot plugins.Snapshot) error {
	if c == nil || c.app == nil || c.app.pluginConfig == nil || len(snapshot.DefaultConfig) == 0 {
		return nil
	}
	_, err := c.app.pluginConfig.SeedDefaults(ctx, snapshot.PluginID, snapshot.DefaultConfig)
	return err
}
