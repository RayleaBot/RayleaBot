package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginkv"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwebhook"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

func buildAppPlugins(
	state appBuildState,
	platform appPlatform,
	renderRunner render.Runner,
) (appPlugins, error) {
	adapterShell := adapter.New(state.core.Config.OneBot, state.core.Logger)
	replyTargets := newReplyTargetCache(defaultReplyTargetCacheSize)
	eventDispatcher := dispatch.New(state.core.Logger, adapterShell, replyTargets, state.core.Config.Runtime.MaxPendingEventsPerPlugin)
	outboundLimiter := outbound.NewMessageRateLimiter(state.core.Config)
	eventDispatcher.SetOutboundLimiter(outboundLimiter)
	eventBridge := bridge.New(state.core.Logger, eventDispatcher)

	pluginRepository, pluginKVRepository, pluginConfigRepository, err := buildPluginRepositories(platform)
	if err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, err
	}
	webhookRegistry := pluginwebhook.NewRegistry()
	pluginFileService := pluginfile.NewService(filepath.Join(filepath.Dir(platform.Storage.Path), "plugins"))
	renderService, err := buildRenderService(state, platform, renderRunner)
	if err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, err
	}
	blacklistRepo := permission.NewSQLiteBlacklistRepository(platform.Storage.Read, platform.Storage.Write)
	whitelistRepo := permission.NewSQLiteWhitelistRepository(platform.Storage.Read, platform.Storage.Write)
	whitelistStateRepo := permission.NewSQLiteWhitelistStateRepository(platform.Storage.Read, platform.Storage.Write)

	if err := hydratePluginCatalog(state, pluginRepository, pluginConfigRepository); err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, err
	}
	cleanupOrphanedInstallDirs(state.core.Logger, state.discoverySpec.roots)
	if err := syncCatalogRenderTemplates(context.Background(), renderService, state.pluginCatalog); err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, err
	}

	pluginInstallService, pluginUninstallService, err := buildPluginMutationServices(state, pluginRepository)
	if err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, err
	}

	return appPlugins{
		Plugins:           state.pluginCatalog,
		Adapter:           adapterShell,
		Bridge:            eventBridge,
		Dispatcher:        eventDispatcher,
		replyTargets:      replyTargets,
		outboundSender:    adapterShell,
		PluginInstaller:   pluginInstallService,
		PluginUninstaller: pluginUninstallService,
		pluginRepository:  pluginRepository,
		pluginConfig:      pluginConfigRepository,
		pluginFiles:       pluginFileService,
		pluginKV:          pluginKVRepository,
		grantRepository:   pluginRepository,
		blacklistRepo:     blacklistRepo,
		whitelistRepo:     whitelistRepo,
		whitelistState:    whitelistStateRepo,
		webhooks:          webhookRegistry,
		renderer:          renderService,
		pluginLogLimiter:  localaction.NewPluginLogLimiter(state.core.Config),
		outboundLimiter:   outboundLimiter,
	}, nil
}

func buildPluginRepositories(platform appPlatform) (*plugins.SQLiteRepository, pluginkv.Repository, pluginconfig.Repository, error) {
	pluginRepository, err := plugins.NewSQLiteRepository(platform.Storage)
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

func buildRenderService(state appBuildState, platform appPlatform, renderRunner render.Runner) (*render.Service, error) {
	renderBrowserPath := prepareRenderBrowserPath(context.Background(), state.core.Logger, state.discoverySpec.repoRoot, state.core.Config.Render.BrowserPath)
	renderService, err := render.NewService(render.Options{
		RepoRoot:           state.discoverySpec.repoRoot,
		OutputRoot:         filepath.Join(filepath.Dir(platform.Storage.Path), "render"),
		Store:              platform.Storage,
		Runner:             renderRunner,
		WorkerCount:        state.core.Config.Render.WorkerCount,
		BrowserArgs:        state.core.Config.Render.BrowserArgs,
		BrowserPath:        renderBrowserPath,
		QueueMaxLength:     state.core.Config.Render.QueueMaxLength,
		QueueWaitTimeout:   time.Duration(state.core.Config.Render.QueueWaitTimeoutSeconds) * time.Second,
		RenderTimeout:      time.Duration(state.core.Config.Render.TimeoutSeconds) * time.Second,
		MaxRenderDataBytes: int(httpapi.MaxManagementJSONBodyBytes),
		Logger:             state.core.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("create render service: %w", err)
	}
	return renderService, nil
}

func hydratePluginCatalog(state appBuildState, pluginRepository *plugins.SQLiteRepository, pluginConfigRepository pluginconfig.Repository) error {
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

func refreshCatalogCommandsFromSettings(ctx context.Context, catalog *plugins.Catalog, repo pluginconfig.Repository) error {
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

func buildPluginMutationServices(state appBuildState, pluginRepository *plugins.SQLiteRepository) (plugins.InstallCoordinator, plugins.UninstallCoordinator, error) {
	pluginInstallService, err := plugins.NewInstallService(
		state.core.Logger,
		state.taskRegistry,
		state.pluginCatalog,
		pluginRepository,
		state.pluginValidator,
		state.discoverySpec.repoRoot,
		state.discoverySpec.roots,
		time.Duration(state.core.Config.Runtime.DependencyInstallTimeoutSecs)*time.Second,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create plugin install service: %w", err)
	}
	pluginUninstallService, err := plugins.NewUninstallService(
		state.core.Logger,
		state.taskRegistry,
		state.pluginCatalog,
		pluginRepository,
		state.pluginValidator,
		state.discoverySpec.repoRoot,
		state.discoverySpec.roots,
		nil,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create plugin uninstall service: %w", err)
	}
	return pluginInstallService, pluginUninstallService, nil
}

func syncCatalogRenderTemplates(ctx context.Context, renderer *render.Service, catalog *plugins.Catalog) error {
	if renderer == nil || catalog == nil {
		return nil
	}
	return renderer.SyncPluginTemplates(ctx, pluginRenderTemplateSources(catalog.List()))
}

func pluginRenderTemplateSources(snapshots []plugins.Snapshot) []render.PluginTemplateSource {
	var sources []render.PluginTemplateSource
	seen := map[string]struct{}{}
	for _, snapshot := range snapshots {
		if !snapshot.Valid || snapshot.RegistrationState != "installed" {
			continue
		}
		for _, declared := range snapshot.RenderTemplates {
			dir, ok := pluginPackageRelativeDir(snapshot.PackageRootPath, declared.Path)
			if !ok {
				continue
			}
			key := snapshot.PluginID + "\x00" + dir
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			sources = append(sources, render.PluginTemplateSource{
				PluginID: snapshot.PluginID,
				Dir:      dir,
			})
		}
	}
	return render.PluginTemplateSourcesFromManifests(sources)
}

func validatePluginRenderTemplates(snapshot plugins.Snapshot) error {
	var sources []render.PluginTemplateSource
	for _, declared := range snapshot.RenderTemplates {
		dir, ok := pluginPackageRelativeDir(snapshot.PackageRootPath, declared.Path)
		if !ok {
			return fmt.Errorf("plugin render template path %q is invalid", declared.Path)
		}
		sources = append(sources, render.PluginTemplateSource{
			PluginID: snapshot.PluginID,
			Dir:      dir,
		})
	}
	return render.ValidatePluginTemplateSources(sources)
}

func pluginPackageRelativeDir(packageRoot, relativePath string) (string, bool) {
	packageRoot = strings.TrimSpace(packageRoot)
	relativePath = strings.TrimSpace(relativePath)
	if packageRoot == "" || relativePath == "" || filepath.IsAbs(relativePath) {
		return "", false
	}
	cleanRelative := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelative == "." || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return "", false
	}
	absoluteRoot, err := filepath.Abs(packageRoot)
	if err != nil {
		return "", false
	}
	candidate := filepath.Join(absoluteRoot, cleanRelative)
	relativeToRoot, err := filepath.Rel(absoluteRoot, candidate)
	if err != nil || relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return "", false
	}
	return candidate, true
}
