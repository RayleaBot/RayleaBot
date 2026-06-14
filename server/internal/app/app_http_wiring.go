package app

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/management/authapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/bilibiliapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/configapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/coreapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/governanceapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/logapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/protocolapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/renderapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/systemapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/taskapi"
	"github.com/RayleaBot/RayleaBot/server/internal/management/thirdpartyapi"
	managementws "github.com/RayleaBot/RayleaBot/server/internal/management/ws"
	lifecyclecommands "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle/commands"
	pluginui "github.com/RayleaBot/RayleaBot/server/internal/plugins/managementui"
)

type appHTTPHandlers struct {
	auth       *authapi.Handlers
	management *coreapi.Handlers
	tasks      *taskapi.Handlers
	eventsWS   *managementws.EventsHandler
}

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
	configHandler := configapi.NewHandlers(configService)
	authHandler := authapi.NewHandlers(authapi.Deps{
		Config:        state,
		Auth:          platformState.Auth,
		LoginFailures: platformState.loginFailures,
	})
	managementHandler := coreapi.NewHandlers(coreapi.Deps{
		Auth:            platformState.Auth,
		System:          services.system,
		RequestShutdown: application.requestShutdown,
	})
	governanceHandler := governanceapi.NewHandlersWithService(services.governance)
	taskHandler := taskapi.NewHandlers(platformState.Tasks, platformState.taskExecutor, pluginState.PluginInstaller)
	logHandler := logapi.NewHandlers(services.logs)
	renderHandler := renderapi.NewHandlers(pluginState.renderer)
	systemHandler := systemapi.NewHandlers(services.system, platformState.Scheduler)
	protocolHandler := protocolapi.NewHandlers(services.protocol)
	thirdPartyHandler := thirdpartyapi.NewThirdPartyHandlers(services.thirdParty, serviceBuild.bilibiliAccountClient, services.bilibiliSource, options.BilibiliHTTPTransport)
	bilibiliHandler := bilibiliapi.NewBilibiliHandlers(services.bilibiliSource, serviceBuild.bilibiliQRLogin, options.BilibiliHTTPTransport)
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
			lifecyclecommands.RefreshPluginCommands(pluginState.Plugins, pluginState.Dispatcher, pluginID, settings)
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
