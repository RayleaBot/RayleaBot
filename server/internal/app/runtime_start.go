package app

import (
	"context"
	"fmt"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/config"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/runtime"
)

type runtimeStarter interface {
	Snapshot() runtime.Snapshot
	Start(context.Context, runtime.Spec, runtime.InitPayload) error
}

func (a *App) handleAdapterEvent(ctx context.Context, event adapter.NormalizedEvent) {
	if a == nil {
		return
	}

	if a.pluginLifecycle != nil {
		a.pluginLifecycle.HandleAdapterEvent(ctx, event)
	}

	if a.Bridge != nil {
		a.Bridge.HandleAdapterEvent(ctx, event)
	}
}

func (a *App) handleAdapterReady(ctx context.Context) {
	if a == nil || a.pluginLifecycle == nil {
		return
	}

	a.pluginLifecycle.HandleAdapterReady(ctx)
}

func ensureRuntimeStartedForEvent(
	ctx context.Context,
	manager runtimeStarter,
	catalog *plugins.Catalog,
	repoRoot string,
	runtimeConfig config.RuntimeConfig,
	event adapter.NormalizedEvent,
) (plugins.Snapshot, bool, error) {
	if event.BotID == "" {
		return plugins.Snapshot{}, false, fmt.Errorf("normalized adapter event is missing bot_id")
	}

	return ensureRuntimeStartedForBot(ctx, manager, catalog, repoRoot, runtimeConfig, event.BotID, nil)
}

func ensureRuntimeStartedForBot(
	ctx context.Context,
	manager runtimeStarter,
	catalog *plugins.Catalog,
	repoRoot string,
	runtimeConfig config.RuntimeConfig,
	botID string,
	grantedCapabilities []string,
) (plugins.Snapshot, bool, error) {
	if manager == nil || catalog == nil {
		return plugins.Snapshot{}, false, nil
	}
	if manager.Snapshot().State != runtime.StateStopped {
		return plugins.Snapshot{}, false, nil
	}
	if botID == "" {
		return plugins.Snapshot{}, false, fmt.Errorf("normalized adapter event is missing bot_id")
	}

	snapshot, ok := selectRuntimeStartupPlugin(catalog, grantedCapabilities)
	if !ok {
		return plugins.Snapshot{}, false, nil
	}

	spec, err := runtime.BuildSpec(snapshot, repoRoot, runtimeConfig)
	if err != nil {
		return snapshot, false, err
	}

	payload := runtime.InitPayload{
		Bot: runtime.BotInfo{
			ID: botID,
		},
		Capabilities: append([]string(nil), grantedCapabilities...),
	}

	if err := manager.Start(ctx, spec, payload); err != nil {
		return snapshot, false, err
	}

	return snapshot, true, nil
}

func selectRuntimeStartupPlugin(catalog *plugins.Catalog, grantedCapabilities []string) (plugins.Snapshot, bool) {
	if catalog == nil {
		return plugins.Snapshot{}, false
	}

	for _, snapshot := range catalog.List() {
		if snapshot.Valid &&
			snapshot.RegistrationState == "installed" &&
			snapshot.DesiredState == "enabled" &&
			len(missingCapabilities(snapshot.RequiredPermissions, grantedCapabilities)) == 0 {
			return snapshot, true
		}
	}

	return plugins.Snapshot{}, false
}
