package lifecycle

import (
	"context"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func persistPluginDesiredState(ctx context.Context, repo plugins.DesiredStateRepository, pluginID, desiredState string) error {
	if repo == nil {
		return nil
	}
	return repo.SaveDesiredState(ctx, pluginID, desiredState, time.Now().UTC())
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

func runtimeInitTimeout(cfg config.RuntimeConfig) time.Duration {
	seconds := cfg.PluginInitMaxTotalSeconds
	if seconds <= 0 {
		seconds = 300
	}
	return time.Duration(seconds+5) * time.Second
}

func (c *Controller) seedPluginDefaultConfig(ctx context.Context, snapshot plugins.Snapshot) error {
	if c == nil || c.pluginConfig == nil || len(snapshot.DefaultConfig) == 0 {
		return nil
	}
	_, err := c.pluginConfig.SeedDefaults(ctx, snapshot.PluginID, snapshot.DefaultConfig)
	return err
}

func (c *Controller) reconcileRecoverySummaryBestEffort(trigger string) {
	if c == nil || c.onRecoveryChange == nil {
		return
	}
	c.onRecoveryChange(trigger)
}
