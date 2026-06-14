package app

import (
	"context"
	"fmt"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginrepository "github.com/RayleaBot/RayleaBot/server/internal/plugins/repository"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginconfig "github.com/RayleaBot/RayleaBot/server/internal/plugins/configstore"
	pluginkv "github.com/RayleaBot/RayleaBot/server/internal/plugins/kvstore"
)

func buildPluginRepositories(platform appPlatform) (*pluginrepository.SQLiteRepository, pluginkv.Repository, pluginconfig.Repository, error) {
	pluginRepository, err := pluginrepository.NewSQLiteRepository(platform.Storage)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create plugin repository: %w", err)
	}
	pluginKVRepository, err := pluginkv.NewSQLiteRepository(platform.Storage)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create plugin kv repository: %w", err)
	}
	pluginConfigRepository, err := pluginconfig.NewSQLiteRepository(platform.Storage)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create plugin config repository: %w", err)
	}
	return pluginRepository, pluginKVRepository, pluginConfigRepository, nil
}

func hydratePluginCatalog(state appBuildState, pluginRepository *pluginrepository.SQLiteRepository, pluginConfigRepository pluginconfig.Repository) error {
	desiredStates, err := pluginRepository.LoadDesiredStates(context.Background())
	if err != nil {
		return fmt.Errorf("load persisted plugin desired_state: %w", err)
	}
	if packageLoader, ok := any(pluginRepository).(plugins.PackageMetadataLoader); ok {
		packageMetadata, err := packageLoader.LoadAllPackageMetadata(context.Background())
		if err != nil {
			return fmt.Errorf("load plugin package metadata: %w", err)
		}
		state.pluginCatalog.Replace(plugins.ApplyPackageMetadata(state.pluginCatalog.List(), packageMetadata))
	}
	state.pluginCatalog.ApplyDesiredStates(desiredStates)
	if err := refreshCatalogCommandsFromSettings(context.Background(), state.pluginCatalog, pluginConfigRepository); err != nil {
		return err
	}
	return nil
}

func refreshCatalogCommandsFromSettings(ctx context.Context, catalog *plugincatalog.Catalog, repo pluginconfig.Repository) error {
	if catalog == nil || repo == nil {
		return nil
	}
	for _, snapshot := range catalog.List() {
		settings := plugins.CloneSettings(snapshot.DefaultConfig)
		persisted, err := repo.ReadAll(ctx, snapshot.PluginID)
		if err != nil {
			return fmt.Errorf("load persisted plugin settings for %s: %w", snapshot.PluginID, err)
		}
		for key, value := range persisted {
			settings[key] = plugins.CloneSettingValue(value)
		}
		catalog.RefreshCommands(snapshot.PluginID, settings)
	}
	return nil
}
