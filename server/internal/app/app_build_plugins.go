package app

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
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

	if err := hydratePluginCatalog(state, pluginRepository); err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, err
	}
	cleanupOrphanedInstallDirs(state.core.Logger, state.discoverySpec.roots)

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

func hydratePluginCatalog(state appBuildState, pluginRepository *plugins.SQLiteRepository) error {
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
