package apphost

import (
	"context"
	"path/filepath"
	"time"

	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/adapter/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwebhook"
	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
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
	if err := pluginservice.SyncCatalogRenderTemplates(context.Background(), renderService, state.pluginCatalog); err != nil {
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
