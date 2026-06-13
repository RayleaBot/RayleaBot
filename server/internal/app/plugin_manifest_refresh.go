package app

import (
	"context"
	"fmt"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginmanifest "github.com/RayleaBot/RayleaBot/server/internal/plugins/manifest"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func refreshPluginManifest(
	ctx context.Context,
	catalog *plugincatalog.Catalog,
	pluginConfig pluginconfigReader,
	pluginID string,
	discover func() ([]plugins.Snapshot, error),
) (plugins.Snapshot, error) {
	if catalog == nil {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	current, ok := catalog.Get(pluginID)
	if !ok {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}
	if discover == nil {
		return current, nil
	}

	discovered, err := discover()
	if err != nil {
		return plugins.Snapshot{}, err
	}
	currentByID := make(map[string]plugins.Snapshot)
	for _, snapshot := range catalog.List() {
		currentByID[snapshot.PluginID] = snapshot
	}
	nextEntries := make([]plugins.Snapshot, 0, len(discovered))
	var refreshed plugins.Snapshot
	found := false
	for _, snapshot := range discovered {
		if existing, ok := currentByID[snapshot.PluginID]; ok {
			snapshot.DesiredState = existing.DesiredState
			snapshot.RuntimeState = existing.RuntimeState
			snapshot.DeadLetter = existing.DeadLetter
			snapshot.PackageSourceType = existing.PackageSourceType
			snapshot.PackageSourceRef = existing.PackageSourceRef
		}
		settings := plugins.CloneSettings(snapshot.DefaultConfig)
		if pluginConfig != nil {
			persisted, err := pluginConfig.ReadAll(ctx, snapshot.PluginID)
			if err != nil {
				return plugins.Snapshot{}, fmt.Errorf("load persisted plugin settings for %s: %w", snapshot.PluginID, err)
			}
			for key, value := range persisted {
				settings[key] = plugins.CloneSettingValue(value)
			}
		}
		snapshot.Commands = pluginmanifest.ProjectCommands(snapshot, settings)
		if snapshot.PluginID == pluginID {
			refreshed = snapshot
			found = true
		}
		nextEntries = append(nextEntries, snapshot)
	}
	if !found {
		return plugins.Snapshot{}, plugins.ErrPluginNotFound
	}

	for _, currentSnapshot := range catalog.List() {
		if currentSnapshot.PluginID == pluginID {
			continue
		}
		known := false
		for _, next := range nextEntries {
			if next.PluginID == currentSnapshot.PluginID {
				known = true
				break
			}
		}
		if !known {
			nextEntries = append(nextEntries, currentSnapshot)
		}
	}

	catalog.Replace(nextEntries)
	return refreshed, nil
}

type pluginconfigReader interface {
	ReadAll(ctx context.Context, pluginID string) (map[string]any, error)
}
