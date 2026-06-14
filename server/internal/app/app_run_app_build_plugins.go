package app

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	pluginfile "github.com/RayleaBot/RayleaBot/server/internal/plugins/filestore"
	lifecyclemetrics "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle/metrics"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeregistry "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/registry"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderbootstrap "github.com/RayleaBot/RayleaBot/server/internal/render/bootstrap"
	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
	renderplugintemplates "github.com/RayleaBot/RayleaBot/server/internal/render/plugintemplates"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
)

const dispatcherRuntimeFlushInterval = 10 * time.Second

func buildAppPlugins(
	state appBuildState,
	platform appPlatform,
	renderRunner renderbrowser.Runner,
) (appPlugins, error) {
	adapterShell := adaptershell.New(state.core.Config.OneBot, state.core.Config.Adapter, state.core.Logger)
	replyTargets := outbound.NewReplyTargetCache(outbound.DefaultReplyTargetCacheSize)
	eventDispatcher := dispatch.New(state.core.Logger, adapterShell, replyTargets, state.core.Config.Runtime.MaxPendingEventsPerPlugin)
	outboundLimiter := outbound.NewMessageRateLimiter(state.core.Config)
	eventDispatcher.SetOutboundLimiter(outboundLimiter)
	eventBridge := bridge.New(state.core.Logger, eventDispatcher)
	eventBridge.SetAdapterStatsSource(adapterShell)
	eventBridge.SetDispatcherStatsSource(metrics.NewDispatcherStatsAdapter(eventDispatcher))
	eventDispatcher.SetRuntimePublisher(metrics.NewDispatcherRuntimePublisher(eventBridge))
	eventDispatcher.StartObservabilityFlush(dispatcherRuntimeFlushInterval)

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
	runtimepaths.CleanupOrphanedInstallDirs(state.core.Logger, state.discoverySpec.Roots)
	if err := renderplugintemplates.SyncCatalogRenderTemplates(context.Background(), renderService, state.pluginCatalog); err != nil {
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

func buildRenderService(state appBuildState, platform appPlatform, renderRunner renderbrowser.Runner) (*renderservice.Service, error) {
	renderBrowserPath := renderbootstrap.PrepareBrowserPath(context.Background(), state.core.Logger, state.discoverySpec.RepoRoot, state.core.Config.Render.BrowserPath)
	renderService, err := renderservice.NewService(renderservice.Options{
		RepoRoot:           state.discoverySpec.RepoRoot,
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
		FooterTemplate:     state.core.Config.Render.FooterTemplate,
		DefaultOutput:      state.core.Config.Render.DefaultOutput,
		DeviceScalePercent: state.core.Config.Render.DeviceScalePercent,
		Logger:             state.core.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("create render service: %w", err)
	}
	return renderService, nil
}

func buildRuntimeRegistryForApp(buildState appBuildState, runtimeState *appRuntimeState, platform appPlatform, localActions *localaction.Service) *runtimeregistry.Registry {
	return runtimeregistry.New(runtimeState.Logger, runtimemanager.Options{
		Console:                    platform.Console,
		RedactText:                 buildState.managementRedact,
		StderrRateLimitBytesPerSec: buildState.core.Config.Runtime.StderrRateLimitBytesPerSec,
		ExecuteLocalAction:         localActions.Execute,
	})
}

func buildBuiltinMenuService(runtimeState *appRuntimeState, pluginStack appPlugins) *menuext.Service {
	return menuext.New(menuext.Deps{
		CurrentConfig: func() config.Config { return runtimeState.Config },
		Plugins:       pluginStack.Plugins,
		Renderer:      pluginStack.renderer,
		Sender:        pluginStack.outboundSender,
		WaitOutbound: func(ctx context.Context, request outbound.MessageLimitRequest) error {
			if pluginStack.outboundLimiter == nil {
				return nil
			}
			return pluginStack.outboundLimiter.Wait(ctx, request)
		},
		Logger: runtimeState.Logger,
	})
}

func wireAppMetrics(platform appPlatform, pluginStack appPlugins) (*metrics.Registry, func()) {
	registry := metrics.New()
	pluginStack.Bridge.SetMetricsObserver(metrics.NewBridgeObserver(registry))
	pluginStack.Dispatcher.SetMetricsObserver(metrics.NewDispatchObserver(registry))
	pluginStack.Adapter.SetMetricsObserver(metrics.NewAdapterObserver(registry))
	platform.taskExecutor.SetMetricsObserver(metrics.NewTaskObserver(registry))
	pluginStack.renderer.SetMetricsObserver(metrics.NewRenderObserver(registry))
	return registry, lifecyclemetrics.StartPluginRuntimeStateGaugeRefresh(registry, pluginStack.Plugins)
}
