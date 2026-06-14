package app

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	managementhttp "github.com/RayleaBot/RayleaBot/server/internal/management/http"
	managementws "github.com/RayleaBot/RayleaBot/server/internal/management/ws"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginui"
)

func configureAppHTTP(application *App, serviceBuild appServiceBuildResult, options Options) {
	state := application.state
	platformState := application.platform
	pluginState := application.pluginStack
	services := application.services

	configService := newConfigHTTPService(configHTTPDeps{
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
	configHandler := managementhttp.NewConfigHandlers(configService)
	authHandler := managementhttp.NewAuthHandlers(managementhttp.AuthDeps{
		Config:        state,
		Auth:          platformState.Auth,
		LoginFailures: platformState.loginFailures,
	})
	managementHandler := managementhttp.NewManagementHandlers(managementhttp.ManagementDeps{
		Auth:            platformState.Auth,
		System:          services.system,
		RequestShutdown: application.requestShutdown,
	})
	governanceHandler := governance.NewHandlersWithService(services.governance)
	taskHandler := managementhttp.NewTaskHandlers(platformState.Tasks, platformState.taskExecutor, pluginState.PluginInstaller)
	logHandler := managementhttp.NewLogHandlers(services.logs)
	renderHandler := managementhttp.NewRenderHandlers(pluginState.renderer)
	systemHandler := managementhttp.NewSystemHandlers(services.system, platformState.Scheduler)
	protocolHandler := managementhttp.NewProtocolHandlers(services.protocol)
	thirdPartyHandler := managementhttp.NewThirdPartyHandlers(services.thirdParty, serviceBuild.bilibiliAccountClient, services.bilibiliSource, options.BilibiliHTTPTransport)
	bilibiliHandler := managementhttp.NewBilibiliHandlers(services.bilibiliSource, serviceBuild.bilibiliQRLogin, options.BilibiliHTTPTransport)
	eventsWS := managementws.NewEventsHandler(pluginState.Bridge, pluginState.Plugins, services.protocol, serviceBuild.status, services.governanceEvents, services.bilibiliEvents)
	tasksWS := managementws.NewTasksHandler(platformState.Tasks)
	logsWS := managementws.NewLogsHandler(services.logs)
	consoleWS := managementws.NewConsoleHandler(platformState.Console, pluginState.Plugins)
	pluginManagementUIHandler := pluginui.NewHandlers(pluginui.Deps{
		Plugins:            pluginState.Plugins,
		PluginConfig:       pluginState.pluginConfig,
		Secrets:            platformState.Secrets,
		NotifyConfigChange: services.localActions.DispatchPluginConfigChanged,
		RefreshCommands: func(ctx context.Context, pluginID string, settings map[string]any) {
			pluginservice.RefreshPluginCommands(pluginState.Plugins, pluginState.Dispatcher, pluginID, settings)
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
