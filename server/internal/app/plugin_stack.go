package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type pluginStackDeps struct {
	Context   context.Context
	Config    config.Config
	Logger    *slog.Logger
	Discovery runtimepaths.PluginDiscoverySpec
	Validator *config.Validator
	Catalog   *plugincatalog.Catalog
	Tasks     *tasks.Registry
	Platform  PlatformState
}

type PluginStackState struct {
	Plugins           *plugincatalog.Catalog
	PluginInstaller   plugins.InstallCoordinator
	PluginUninstaller plugins.UninstallCoordinator
	PluginRepository  plugins.DesiredStateRepository
	PluginConfig      pluginstore.ConfigRepository
	PluginFiles       *pluginstore.FileService
	PluginKV          pluginstore.KVRepository
	Webhooks          *pluginwebhook.Registry
	PluginLogLimiter  *localaction.PluginLogLimiter
	RefreshManifest   func(context.Context, string) (plugins.Snapshot, error)
}

func buildPluginStack(deps pluginStackDeps) (PluginStackState, error) {
	ctx := deps.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return PluginStackState{}, err
	}

	pluginRepository, pluginKVRepository, pluginConfigRepository, err := buildPluginRepositories(deps.Platform)
	if err != nil {
		return PluginStackState{}, err
	}
	webhookRegistry := pluginwebhook.NewRegistry()
	pluginFileService := pluginstore.NewFileService(filepath.Join(filepath.Dir(deps.Platform.Storage.Path), "plugins"))

	if err := hydratePluginCatalog(ctx, deps.Catalog, pluginRepository, pluginConfigRepository); err != nil {
		return PluginStackState{}, err
	}
	runtimepaths.CleanupOrphanedInstallDirs(deps.Logger, deps.Discovery.Roots)

	pluginInstallService, pluginUninstallService, err := buildPluginMutationServices(deps, pluginRepository)
	if err != nil {
		return PluginStackState{}, err
	}

	return PluginStackState{
		Plugins:           deps.Catalog,
		PluginInstaller:   pluginInstallService,
		PluginUninstaller: pluginUninstallService,
		PluginRepository:  pluginRepository,
		PluginConfig:      pluginConfigRepository,
		PluginFiles:       pluginFileService,
		PluginKV:          pluginKVRepository,
		Webhooks:          webhookRegistry,
		PluginLogLimiter:  localaction.NewPluginLogLimiter(deps.Config),
		RefreshManifest:   buildManifestRefresh(deps, pluginRepository, pluginConfigRepository),
	}, nil
}

func buildManifestRefresh(
	deps pluginStackDeps,
	pluginRepository plugins.DesiredStateRepository,
	pluginConfigRepository pluginstore.ConfigRepository,
) func(context.Context, string) (plugins.Snapshot, error) {
	return func(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
		return pluginservice.RefreshPluginManifest(ctx, deps.Catalog, pluginConfigRepository, pluginID, func() ([]plugins.Snapshot, error) {
			snapshots, _, err := plugincatalog.Discover(plugincatalog.DiscoverOptions{
				Validator: deps.Validator,
				Roots:     deps.Discovery.Roots,
				RepoRoot:  deps.Discovery.RepoRoot,
				Logger:    deps.Logger,
			})
			if err != nil {
				return nil, err
			}
			if packageLoader, ok := any(pluginRepository).(plugins.PackageMetadataLoader); ok {
				packageMetadata, err := packageLoader.LoadAllPackageMetadata(ctx)
				if err != nil {
					return nil, err
				}
				snapshots = plugins.ApplyPackageMetadata(snapshots, packageMetadata)
			}
			return snapshots, nil
		})
	}
}

func buildPluginRepositories(platform PlatformState) (*plugins.SQLiteRepository, pluginstore.KVRepository, pluginstore.ConfigRepository, error) {
	pluginRepository, err := plugins.NewSQLiteRepository(platform.Storage)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create plugin repository: %w", err)
	}
	pluginKVRepository, err := pluginstore.NewKVSQLiteRepository(platform.Storage)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create plugin kv repository: %w", err)
	}
	pluginConfigRepository, err := pluginstore.NewConfigSQLiteRepository(platform.Storage)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create plugin config repository: %w", err)
	}
	return pluginRepository, pluginKVRepository, pluginConfigRepository, nil
}

func hydratePluginCatalog(ctx context.Context, catalog *plugincatalog.Catalog, pluginRepository *plugins.SQLiteRepository, pluginConfigRepository pluginstore.ConfigRepository) error {
	desiredStates, err := pluginRepository.LoadDesiredStates(ctx)
	if err != nil {
		return fmt.Errorf("load persisted plugin desired_state: %w", err)
	}
	if packageLoader, ok := any(pluginRepository).(plugins.PackageMetadataLoader); ok {
		packageMetadata, err := packageLoader.LoadAllPackageMetadata(ctx)
		if err != nil {
			return fmt.Errorf("load plugin package metadata: %w", err)
		}
		catalog.Replace(plugins.ApplyPackageMetadata(catalog.List(), packageMetadata))
	}
	catalog.ApplyDesiredStates(desiredStates)
	if err := refreshCatalogCommandsFromSettings(ctx, catalog, pluginConfigRepository); err != nil {
		return err
	}
	return nil
}

func refreshCatalogCommandsFromSettings(ctx context.Context, catalog *plugincatalog.Catalog, repo pluginstore.ConfigRepository) error {
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

func buildPluginMutationServices(deps pluginStackDeps, pluginRepository *plugins.SQLiteRepository) (plugins.InstallCoordinator, plugins.UninstallCoordinator, error) {
	pluginInstallService, err := pluginservice.NewInstallService(
		deps.Logger,
		deps.Tasks,
		deps.Catalog,
		pluginRepository,
		deps.Validator,
		deps.Discovery.RepoRoot,
		deps.Discovery.Roots,
		time.Duration(deps.Config.Runtime.DependencyInstallTimeoutSecs)*time.Second,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create plugin install service: %w", err)
	}
	pluginUninstallService, err := pluginservice.NewUninstallService(
		deps.Logger,
		deps.Tasks,
		deps.Catalog,
		pluginRepository,
		deps.Validator,
		deps.Discovery.RepoRoot,
		deps.Discovery.Roots,
		nil,
	)
	if err != nil {
		closeErr := pluginInstallService.Close()
		return nil, nil, errors.Join(fmt.Errorf("create plugin uninstall service: %w", err), closeErr)
	}
	return pluginInstallService, pluginUninstallService, nil
}
