package lifecycle

import (
	"context"
	"fmt"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginstore "github.com/RayleaBot/RayleaBot/server/internal/plugins/storage"
)

type PluginConfigReader interface {
	ReadAll(ctx context.Context, pluginID string) (map[string]any, error)
}

func RefreshPluginManifest(
	ctx context.Context,
	catalog *plugincatalog.Catalog,
	pluginConfig PluginConfigReader,
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
	for _, snapshot := range discovered {
		if snapshot.PluginID != pluginID {
			continue
		}
		snapshot.PackageSourceType = current.PackageSourceType
		snapshot.PackageSourceRef = current.PackageSourceRef
		effective := pluginstore.MergeValues(snapshot.DefaultConfig, nil)
		if pluginConfig != nil {
			persisted, err := pluginConfig.ReadAll(ctx, pluginID)
			if err != nil {
				return plugins.Snapshot{}, fmt.Errorf("load persisted plugin settings for %s: %w", pluginID, err)
			}
			effective = pluginstore.MergeValues(snapshot.DefaultConfig, persisted)
		}
		snapshot.Commands = plugincatalog.ProjectCommands(snapshot, effective)
		catalog.RefreshInstalled([]plugins.Snapshot{snapshot}, pluginID)
		updated, ok := catalog.Get(pluginID)
		if !ok {
			return plugins.Snapshot{}, plugins.ErrPluginNotFound
		}
		return updated, nil
	}
	return plugins.Snapshot{}, plugins.ErrPluginNotFound
}
