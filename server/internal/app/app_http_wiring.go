package app

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginui"
)

func configureAppHTTP(application *App, serviceBuild appServiceBuildResult, options Options) {
	state := application.state
	platformState := application.platform
	pluginState := application.pluginStack
	services := application.services

	configHandler := newConfigHTTPHandlers(configHTTPDeps{
		state:            state,
		logs:             platformState.Logs,
		logRepository:    platformState.LogRepository,
		renderer:         pluginState.renderer,
		pluginLogLimiter: pluginState.pluginLogLimiter,
		outboundLimiter:  pluginState.outboundLimiter,
		protocol:         services.protocol,
		eventIngress:     services.eventIngress,
		blacklistRepo:    pluginState.blacklistRepo,
	})
	authHandler := newAuthHTTPHandlers(authHTTPDeps{
		config:        state,
		auth:          platformState.Auth,
		loginFailures: platformState.loginFailures,
	})
	managementHandler := newManagementHTTPHandlers(managementHTTPDeps{
		auth:            platformState.Auth,
		system:          services.system,
		requestShutdown: application.requestShutdown,
	})
	governanceHandler := governance.NewHandlersWithService(services.governance)
	taskHandler := newTaskHTTPHandlers(platformState.Tasks, platformState.taskExecutor, pluginState.PluginInstaller)
	logHandler := newLogHTTPHandlers(services.logs)
	renderHandler := newRenderHTTPHandlers(pluginState.renderer)
	systemHandler := newSystemHTTPHandlers(services.system, platformState.Scheduler)
	protocolHandler := newProtocolHTTPHandlers(services.protocol)
	thirdPartyHandler := newThirdPartyHTTPHandlers(services.thirdParty, serviceBuild.bilibiliAccountClient, services.bilibiliSource, options.BilibiliHTTPTransport)
	bilibiliHandler := newBilibiliSourceHTTPHandlers(services.bilibiliSource, serviceBuild.bilibiliQRLogin, options.BilibiliHTTPTransport)
	eventsWS := newEventsWSHandler(pluginState.Bridge, pluginState.Plugins, services.protocol, serviceBuild.status, services.governanceEvents, services.bilibiliEvents)
	tasksWS := newTasksWSHandler(platformState.Tasks)
	logsWS := newLogsWSHandler(services.logs)
	consoleWS := newConsoleWSHandler(platformState.Console, pluginState.Plugins)
	pluginManagementUIHandler := pluginui.NewHandlers(pluginui.Deps{
		Plugins:            pluginState.Plugins,
		PluginConfig:       pluginState.pluginConfig,
		Secrets:            platformState.Secrets,
		NotifyConfigChange: services.localActions.DispatchPluginConfigChanged,
		RefreshCommands: func(ctx context.Context, pluginID string, settings map[string]any) {
			applicationRefreshPluginCommands(pluginState.Plugins, pluginState.Dispatcher, pluginID, settings)
		},
	})

	router, server := buildAppHTTPServer(httpServerDeps{
		state:              state,
		auth:               platformState.Auth,
		tasks:              platformState.Tasks,
		plugins:            pluginState.Plugins,
		pluginInstaller:    pluginState.PluginInstaller,
		pluginUninstaller:  pluginState.PluginUninstaller,
		pluginRepository:   pluginState.pluginRepository,
		grantRepository:    pluginState.grantRepository,
		pluginLifecycle:    services.pluginLifecycle,
		renderer:           pluginState.renderer,
		configHandler:      configHandler,
		authHandler:        authHandler,
		managementHandler:  managementHandler,
		governanceHandler:  governanceHandler,
		taskHandler:        taskHandler,
		logHandler:         logHandler,
		renderHandler:      renderHandler,
		systemHandler:      systemHandler,
		protocolHandler:    protocolHandler,
		thirdPartyHandler:  thirdPartyHandler,
		bilibiliHandler:    bilibiliHandler,
		eventsWS:           eventsWS,
		tasksWS:            tasksWS,
		logsWS:             logsWS,
		consoleWS:          consoleWS,
		pluginWebhooks:     services.pluginWebhooks,
		pluginManagementUI: pluginManagementUIHandler,
		metrics:            application.metrics,
	})
	application.process.router = router
	application.process.server = server
	application.httpHandlers = appHTTPHandlers{
		auth:       authHandler,
		management: managementHandler,
		tasks:      taskHandler,
		eventsWS:   eventsWS,
	}
}
