package app

import (
	"context"
	"strings"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
	"github.com/RayleaBot/RayleaBot/server/internal/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/runtime/manager"
	runtimespec "github.com/RayleaBot/RayleaBot/server/internal/runtime/spec"
)

type runtimeStarter interface {
	Snapshot() runtimemanager.Snapshot
	Start(context.Context, runtimespec.Spec, runtimespec.InitPayload) error
}

func ensureRuntimeStartedForEvent(
	ctx context.Context,
	manager runtimeStarter,
	catalog *plugincatalog.Catalog,
	repoRoot string,
	cfg config.Config,
	event adapterintake.NormalizedEvent,
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
	if manager.Snapshot().State != runtimemanager.StateStopped {
		return plugins.Snapshot{}, false, nil
	}
	botID = strings.TrimSpace(botID)

	snapshot, ok := selectRuntimeStartupPlugin(catalog, grantedCapabilities)
	if !ok {
		return plugins.Snapshot{}, false, nil
	}

	spec, err := runtimespec.BuildSpec(snapshot, repoRoot, cfg.Runtime)
	if err != nil {
		return snapshot, false, err
	}

	payload := runtimespec.InitPayload{
		Bot: runtimespec.BotInfo{
			ID: botID,
		},
		Capabilities:    append([]string(nil), grantedCapabilities...),
		SuperAdmins:     pluginservice.PluginRuntimeSuperAdmins(cfg),
		CommandPrefixes: chatpolicy.RuntimeCommandPrefixes(cfg),
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
