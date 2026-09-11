package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/market"
	pluginstore "github.com/RayleaBot/RayleaBot/server/internal/plugins/storage"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	"github.com/RayleaBot/RayleaBot/server/internal/releaseupdate"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

type pluginStackDeps struct {
	Context   context.Context
	Config    config.Config
	Logger    *slog.Logger
	Discovery plugincatalog.DiscoverySpec
	Validator *config.Validator
	Catalog   *plugincatalog.Catalog
	Tasks     *tasks.Registry
	Platform  PlatformState
}

type PluginStackState struct {
	Operations        *pluginservice.OperationGate
	Plugins           *plugincatalog.Catalog
	PluginInstaller   *pluginservice.InstallService
	PluginStore       market.ServiceAPI
	PluginUninstaller *pluginservice.UninstallService
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

	return PluginStackState{
		Operations:       pluginservice.NewOperationGate(),
		Plugins:          deps.Catalog,
		PluginRepository: pluginRepository,
		PluginConfig:     pluginConfigRepository,
		PluginFiles:      pluginFileService,
		PluginKV:         pluginKVRepository,
		Webhooks:         webhookRegistry,
		PluginLogLimiter: localaction.NewPluginLogLimiter(deps.Config),
		RefreshManifest:  buildManifestRefresh(deps, pluginRepository, pluginConfigRepository),
	}, nil
}

func buildManifestRefresh(
	deps pluginStackDeps,
	pluginRepository plugins.PackageMetadataLoader,
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
			packageMetadata, err := pluginRepository.LoadAllPackageMetadata(ctx)
			if err != nil {
				return nil, err
			}
			return plugins.ApplyPackageMetadata(snapshots, packageMetadata), nil
		})
	}
}

func buildPluginRepositories(platform PlatformState) (*plugincatalog.SQLiteRepository, pluginstore.KVRepository, pluginstore.ConfigRepository, error) {
	pluginRepository, err := plugincatalog.NewSQLiteRepository(platform.Storage)
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

func hydratePluginCatalog(ctx context.Context, catalog *plugincatalog.Catalog, pluginRepository *plugincatalog.SQLiteRepository, pluginConfigRepository pluginstore.ConfigRepository) error {
	desiredStates, err := pluginRepository.LoadDesiredStates(ctx)
	if err != nil {
		return fmt.Errorf("load persisted plugin desired_state: %w", err)
	}
	packageMetadata, err := pluginRepository.LoadAllPackageMetadata(ctx)
	if err != nil {
		return fmt.Errorf("load plugin package metadata: %w", err)
	}
	catalog.Replace(plugins.ApplyPackageMetadata(catalog.List(), packageMetadata))
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
		persisted, err := repo.ReadAll(ctx, snapshot.PluginID)
		if err != nil {
			return fmt.Errorf("load persisted plugin settings for %s: %w", snapshot.PluginID, err)
		}
		settings := pluginstore.MergeValues(snapshot.DefaultConfig, persisted)
		catalog.RefreshCommands(snapshot.PluginID, settings)
	}
	return nil
}

func buildPluginMutationServices(deps pluginStackDeps, state *PluginStackState, services Services, renderer *render.Service) error {
	pluginRepository := state.PluginRepository
	if state.Operations == nil || services.PluginLifecycle == nil || services.System == nil || services.PluginWebhooks == nil {
		return errors.New("plugin mutations require the constructed lifecycle and services")
	}
	reconcileInstalled := func(ctx context.Context, pluginID string) error {
		services.PluginWebhooks.SyncManifestRegistrations()
		if err := syncCatalogRenderTemplates(ctx, renderer, state.Plugins); err != nil {
			return err
		}
		if snapshot, exists := state.Plugins.Get(pluginID); exists && snapshot.DesiredState == plugins.DesiredStateEnabled {
			if err := services.PluginLifecycle.StartInstalled(ctx, pluginID); err != nil {
				return err
			}
		}
		services.System.ReconcileRecoverySummaryBestEffort("plugin.install")
		return nil
	}
	pluginInstallService, err := pluginservice.NewInstallService(
		deps.Logger,
		deps.Tasks,
		deps.Catalog,
		pluginRepository,
		deps.Validator,
		deps.Discovery.RepoRoot,
		deps.Discovery.Roots,
		0,
		pluginservice.InstallOptions{Operations: state.Operations, BeforeReplace: services.PluginLifecycle.StopAndResetPluginWithContext,
			AfterSuccess: reconcileInstalled, AfterRollback: reconcileInstalled, ValidateRenderTemplates: validatePluginRenderTemplates},
	)
	if err != nil {
		return fmt.Errorf("create plugin install service: %w", err)
	}
	state.PluginInstaller = pluginInstallService
	pluginUninstallService, err := pluginservice.NewUninstallService(
		deps.Logger,
		deps.Tasks,
		deps.Catalog,
		pluginRepository,
		deps.Validator,
		deps.Discovery.RepoRoot,
		deps.Discovery.Roots,
		pluginservice.UninstallOptions{Operations: state.Operations, StopPlugin: services.PluginLifecycle.StopAndResetPluginWithContext,
			AfterSuccess: func(ctx context.Context, pluginID string) error {
				services.PluginWebhooks.SyncManifestRegistrations()
				var cleanupErr error
				if deps.Platform.Scheduler != nil {
					cleanupErr = deps.Platform.Scheduler.UnregisterByPlugin(ctx, pluginID)
				}
				if renderer != nil {
					cleanupErr = errors.Join(cleanupErr, renderer.RemovePluginTemplates(ctx, pluginID))
				}
				cleanupErr = errors.Join(cleanupErr, syncCatalogRenderTemplates(ctx, renderer, state.Plugins))
				services.System.ReconcileRecoverySummaryBestEffort("plugin.uninstall")
				return cleanupErr
			}},
	)
	if err != nil {
		return fmt.Errorf("create plugin uninstall service: %w", err)
	}
	state.PluginUninstaller = pluginUninstallService
	pluginStoreRepository, err := market.NewSQLiteRepository(deps.Platform.Storage)
	if err != nil {
		return fmt.Errorf("create plugin store repository: %w", err)
	}
	state.PluginStore, err = market.New(deps.Context, state.Plugins, state.PluginInstaller, pluginStoreRepository, market.Options{CoreVersion: releaseupdate.InstalledVersion(deps.Discovery.RepoRoot)})
	return err
}
