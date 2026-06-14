package apphost

import (
	"context"
	"time"

	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventingress"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	managementhttp "github.com/RayleaBot/RayleaBot/server/internal/management/http"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
	runtimeregistry "github.com/RayleaBot/RayleaBot/server/internal/runtime/registry"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type appServiceBuildResult struct {
	services              appServices
	runtimes              *runtimeregistry.Registry
	status                *managementevents.ServiceStatusService
	bilibiliAccountClient *bilibilisession.AccountClient
	bilibiliQRLogin       *bilibilisession.QRLoginService
}

func buildAppServices(
	buildState appBuildState,
	runtimeState *appRuntimeState,
	platform appPlatform,
	pluginStack appPlugins,
	metricRegistry *metrics.Registry,
	options Options,
) (appServiceBuildResult, error) {
	logService := logging.NewManagementService(platform.Logs, platform.LogRepository)
	grantView := buildPluginGrantView(runtimeState, pluginStack)
	governanceEvents := managementevents.NewGovernanceService()
	bilibiliEvents := managementevents.NewBilibiliSourceService()
	governanceService := buildGovernanceService(runtimeState, pluginStack, governanceEvents)
	thirdPartyService, err := thirdparty.NewService(platform.Storage, platform.Secrets)
	if err != nil {
		return appServiceBuildResult{}, err
	}
	bilibiliSession := bilibilisession.NewSessionClient(options.BilibiliHTTPTransport, options.BilibiliClock, nil)
	localActions := buildLocalActionService(runtimeState, platform, pluginStack, grantView, governanceService, thirdPartyService, bilibiliSession)
	configureLocalActionService(localActions, pluginStack)
	runtimeRegistry := buildRuntimeRegistryForApp(buildState, runtimeState, platform, localActions)
	systemService := systemsvc.New(systemsvc.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		CurrentSummary:   func() config.Summary { return runtimeState.Summary },
		CurrentRepoRoot:  func() string { return runtimeState.repoRoot },
		CurrentStartedAt: func() time.Time { return runtimeState.startedAt },
		Logger:           runtimeState.Logger,
		Auth:             platform.Auth,
		Adapter:          pluginStack.Adapter,
		Plugins:          pluginStack.Plugins,
		Runtimes:         runtimeRegistry,
		Renderer:         pluginStack.renderer,
		Storage:          platform.Storage,
		PluginRepository: pluginStack.pluginRepository,
		TaskExecutor:     platform.taskExecutor,
		LogRepository:    platform.LogRepository,
	})
	serviceStatusService := managementevents.NewServiceStatusService(systemService)
	systemService.SetStatusPublisher(serviceStatusService)
	lifecycle := pluginservice.NewController(pluginservice.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		RepoRoot:         runtimeState.repoRoot,
		Logger:           runtimeState.Logger,
		Plugins:          pluginStack.Plugins,
		DesiredStateRepo: pluginStack.pluginRepository,
		Grants:           grantView,
		Runtimes:         runtimeRegistry,
		Dispatcher:       pluginStack.Dispatcher,
		Scheduler:        platform.Scheduler,
		PluginConfig:     pluginStack.pluginConfig,
		Adapter:          pluginStack.Adapter,
		Webhooks:         pluginStack.webhooks,
		Tasks:            platform.Tasks,
		OnRecoveryChange: systemService.ReconcileRecoverySummaryBestEffort,
		RefreshManifest:  buildPluginLifecycleRefreshManifest(buildState, runtimeState, pluginStack),
		SyncRenderTemplates: func(ctx context.Context) error {
			return pluginservice.SyncCatalogRenderTemplates(ctx, pluginStack.renderer, pluginStack.Plugins)
		},
	})
	menuService := buildBuiltinMenuService(runtimeState, pluginStack)
	eventIngress := eventingress.New(eventingress.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		Logger:           runtimeState.Logger,
		Plugins:          pluginStack.Plugins,
		ReplyTargets:     pluginStack.replyTargets,
		OutboundSender:   pluginStack.outboundSender,
		OutboundLimiter:  pluginStack.outboundLimiter,
		Renderer:         pluginStack.renderer,
		Menu:             menuService,
		Bridge:           pluginStack.Bridge,
		Lifecycle:        lifecycle,
		MetadataEnricher: pluginStack.Adapter,
		WhitelistRepo:    pluginStack.whitelistRepo,
		WhitelistState:   pluginStack.whitelistState,
		BlacklistRepo:    pluginStack.blacklistRepo,
	})
	protocolService := managementhttp.NewProtocolService(runtimeState, pluginStack.Adapter)
	pluginWebhooks := buildPluginWebhookGateway(runtimeState, platform, pluginStack, lifecycle, grantView)
	pluginWebhooks.SetReplayMetrics(metrics.NewWebhookReplayObserver(metricRegistry))
	localActions.SetWebhookGateway(pluginWebhooks)
	bilibiliSource, err := buildBilibiliSourceService(platform, pluginStack, thirdPartyService, bilibiliSession, bilibiliEvents, options)
	if err != nil {
		return appServiceBuildResult{}, err
	}

	return appServiceBuildResult{
		services: appServices{
			localActions:     localActions,
			pluginLifecycle:  lifecycle,
			eventIngress:     eventIngress,
			protocol:         protocolService,
			pluginWebhooks:   pluginWebhooks,
			governance:       governanceService,
			governanceEvents: governanceEvents,
			logs:             logService,
			system:           systemService,
			thirdParty:       thirdPartyService,
			bilibiliSource:   bilibiliSource,
			bilibiliEvents:   bilibiliEvents,
		},
		runtimes:              runtimeRegistry,
		status:                serviceStatusService,
		bilibiliAccountClient: bilibilisession.NewAccountClient(options.BilibiliHTTPTransport, options.BilibiliClock, nil),
		bilibiliQRLogin:       bilibilisession.NewQRLoginService(options.BilibiliHTTPTransport, options.BilibiliClock),
	}, nil
}
