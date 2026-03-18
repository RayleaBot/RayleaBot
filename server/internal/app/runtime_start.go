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

	if snapshot, started, err := ensureRuntimeStartedForEvent(ctx, a.Runtime, a.Plugins, a.repoRoot, a.Config.Runtime, event); err != nil {
		if a.Logger != nil {
			a.Logger.Warn(
				"plugin runtime startup failed before adapter event delivery",
				"component", "app",
				"plugin_id", snapshot.PluginID,
				"event_id", event.EventID,
				"event_type", event.EventType,
				"err", err.Error(),
			)
		}
	} else if started && a.Logger != nil {
		a.Logger.Info(
			"plugin runtime started for adapter event bridge",
			"component", "app",
			"plugin_id", snapshot.PluginID,
			"event_id", event.EventID,
			"event_type", event.EventType,
		)
	}

	if a.Bridge != nil {
		a.Bridge.HandleAdapterEvent(ctx, event)
	}
}

func ensureRuntimeStartedForEvent(
	ctx context.Context,
	manager runtimeStarter,
	catalog *plugins.Catalog,
	repoRoot string,
	runtimeConfig config.RuntimeConfig,
	event adapter.NormalizedEvent,
) (plugins.Snapshot, bool, error) {
	if manager == nil || catalog == nil {
		return plugins.Snapshot{}, false, nil
	}
	if manager.Snapshot().State != runtime.StateStopped {
		return plugins.Snapshot{}, false, nil
	}
	if event.BotID == "" {
		return plugins.Snapshot{}, false, fmt.Errorf("normalized adapter event is missing bot_id")
	}

	snapshot, ok := selectRuntimeStartupPlugin(catalog)
	if !ok {
		return plugins.Snapshot{}, false, nil
	}

	spec, err := runtime.BuildSpec(snapshot, repoRoot, runtimeConfig)
	if err != nil {
		return snapshot, false, err
	}

	payload := runtime.InitPayload{
		Bot: runtime.BotInfo{
			ID: event.BotID,
		},
	}

	if err := manager.Start(ctx, spec, payload); err != nil {
		return snapshot, false, err
	}

	return snapshot, true, nil
}

func selectRuntimeStartupPlugin(catalog *plugins.Catalog) (plugins.Snapshot, bool) {
	if catalog == nil {
		return plugins.Snapshot{}, false
	}

	for _, snapshot := range catalog.List() {
		if snapshot.Valid {
			return snapshot, true
		}
	}

	return plugins.Snapshot{}, false
}
