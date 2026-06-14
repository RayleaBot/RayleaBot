package app

import (
	"context"
	"strings"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

type runtimeStarter interface {
	Snapshot() runtime.Snapshot
	Start(context.Context, runtime.Spec, runtime.InitPayload) error
}

func ensureRuntimeStartedForEvent(
	ctx context.Context,
	manager runtimeStarter,
	catalog *plugincatalog.Catalog,
	repoRoot string,
	cfg config.Config,
	event adapter.NormalizedEvent,
) (plugins.Snapshot, bool, error) {
	return ensureRuntimeStartedForBot(ctx, manager, catalog, repoRoot, cfg, strings.TrimSpace(event.BotID), nil)
}

func ensureRuntimeStartedForBot(
	ctx context.Context,
	manager runtimeStarter,
	catalog *plugincatalog.Catalog,
	repoRoot string,
	cfg config.Config,
	botID string,
	grantedCapabilities []string,
) (plugins.Snapshot, bool, error) {
	if manager == nil || catalog == nil {
		return plugins.Snapshot{}, false, nil
	}
	if manager.Snapshot().State != runtime.StateStopped {
		return plugins.Snapshot{}, false, nil
	}
	botID = strings.TrimSpace(botID)

	snapshot, ok := selectRuntimeStartupPlugin(catalog, grantedCapabilities)
	if !ok {
		return plugins.Snapshot{}, false, nil
	}

	spec, err := runtime.BuildSpec(snapshot, repoRoot, cfg.Runtime)
	if err != nil {
		return snapshot, false, err
	}

	payload := runtime.InitPayload{
		Bot: runtime.BotInfo{
			ID: botID,
		},
		Capabilities:    append([]string(nil), grantedCapabilities...),
		SuperAdmins:     pluginservice.PluginRuntimeSuperAdmins(cfg),
		CommandPrefixes: runtimeCommandPrefixes(cfg),
	}

	if err := manager.Start(ctx, spec, payload); err != nil {
		return snapshot, false, err
	}

	return snapshot, true, nil
}

func selectRuntimeStartupPlugin(catalog *plugincatalog.Catalog, grantedCapabilities []string) (plugins.Snapshot, bool) {
	if catalog == nil {
		return plugins.Snapshot{}, false
	}

	for _, snapshot := range catalog.List() {
		if snapshot.Valid &&
			snapshot.RegistrationState == "installed" &&
			snapshot.DesiredState == "enabled" &&
			len(pluginservice.MissingCapabilities(snapshot.RequiredPermissions, grantedCapabilities)) == 0 {
			return snapshot, true
		}
	}

	return plugins.Snapshot{}, false
}
