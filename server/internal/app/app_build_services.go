package app

import (
	"context"

	source "github.com/RayleaBot/RayleaBot/server/internal/bilibili"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type appServiceBuildResult struct {
	services              appServices
	runtimes              *runtimeRegistry
	status                *serviceStatusService
	bilibiliAccountClient *source.AccountClient
	bilibiliQRLogin       *source.QRLoginService
}

func buildAppServices(
	buildState appBuildState,
	runtimeState *appRuntimeState,
	platform appPlatform,
	pluginStack appPlugins,
	metricRegistry *metrics.Registry,
	options Options,
) (appServiceBuildResult, error) {
	logService := newLogService(platform.Logs, platform.LogRepository)
	grantView := buildPluginGrantView(runtimeState, pluginStack)
	governanceEvents := newGovernanceEventService()
	bilibiliEvents := newBilibiliSourceEventService()
	governanceService := buildGovernanceService(runtimeState, pluginStack, governanceEvents)
	thirdPartyService, err := thirdparty.NewService(platform.Storage, platform.Secrets)
	if err != nil {
		return appServiceBuildResult{}, err
	}
	bilibiliSession := source.NewSessionClient(options.BilibiliHTTPTransport, options.BilibiliClock, nil)
	localActions := buildLocalActionService(runtimeState, platform, pluginStack, grantView, governanceService, thirdPartyService, bilibiliSession)
	configureLocalActionService(localActions, pluginStack)
	runtimeRegistry := buildRuntimeRegistryForApp(buildState, runtimeState, platform, localActions)
	systemService := newSystemService(systemServiceDeps{
		state:            runtimeState,
		auth:             platform.Auth,
		adapter:          pluginStack.Adapter,
		plugins:          pluginStack.Plugins,
		runtimes:         runtimeRegistry,
		renderer:         pluginStack.renderer,
		storage:          platform.Storage,
		pluginRepository: pluginStack.pluginRepository,
		taskExecutor:     platform.taskExecutor,
		logRepository:    platform.LogRepository,
	})
	serviceStatusService := newServiceStatusService(systemService)
	systemService.statusPublisher = serviceStatusService
	lifecycle := newPluginLifecycleController(pluginLifecycleDeps{
		state:            runtimeState,
		plugins:          pluginStack.Plugins,
		desiredStateRepo: pluginStack.pluginRepository,
		grants:           grantView,
		runtimes:         runtimeRegistry,
		dispatcher:       pluginStack.Dispatcher,
		scheduler:        platform.Scheduler,
		pluginConfig:     pluginStack.pluginConfig,
		adapter:          pluginStack.Adapter,
		webhooks:         pluginStack.webhooks,
		tasks:            platform.Tasks,
		onRecoveryChange: systemService.ReconcileRecoverySummaryBestEffort,
		refreshManifest:  buildPluginLifecycleRefreshManifest(buildState, runtimeState, pluginStack),
		syncRenderTemplates: func(ctx context.Context) error {
			return syncCatalogRenderTemplates(ctx, pluginStack.renderer, pluginStack.Plugins)
		},
	})
	menuService := buildBuiltinMenuService(runtimeState, pluginStack)
	eventIngress := newEventIngressService(eventIngressDeps{
		state:            runtimeState,
		plugins:          pluginStack.Plugins,
		replyTargets:     pluginStack.replyTargets,
		outboundSender:   pluginStack.outboundSender,
		outboundLimiter:  pluginStack.outboundLimiter,
		renderer:         pluginStack.renderer,
		menu:             menuService,
		bridge:           pluginStack.Bridge,
		lifecycle:        lifecycle,
		metadataEnricher: pluginStack.Adapter,
		whitelistRepo:    pluginStack.whitelistRepo,
		whitelistState:   pluginStack.whitelistState,
		blacklistRepo:    pluginStack.blacklistRepo,
	})
	protocolService := newProtocolService(runtimeState, pluginStack.Adapter)
	pluginWebhooks := buildPluginWebhookGateway(runtimeState, platform, pluginStack, lifecycle, grantView)
	pluginWebhooks.SetReplayMetrics(webhookReplayMetricsAdapter{registry: metricRegistry})
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
		bilibiliAccountClient: source.NewAccountClient(options.BilibiliHTTPTransport, options.BilibiliClock, nil),
		bilibiliQRLogin:       source.NewQRLoginService(options.BilibiliHTTPTransport, options.BilibiliClock),
	}, nil
}
