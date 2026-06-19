package httpwire

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/app/servicegraph"
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

func Build(deps BuildDeps) State {
	runtimeState := deps.Runtime
	platformState := deps.Platform
	pluginState := deps.Plugins
	services := deps.Services

	configService := NewConfigService(ConfigDeps{
		Runtime:          runtimeState,
		Logs:             platformState.Logs,
		LogRepository:    platformState.LogRepository,
		Renderer:         pluginState.Renderer,
		PluginLogLimiter: pluginState.PluginLogLimiter,
		OutboundLimiter:  pluginState.OutboundLimiter,
		Protocol:         services.Protocol,
		EventIngress:     services.EventIngress,
	})
	configHandler := configapi.NewHandlers(configService)
	authHandler := authapi.NewHandlers(authapi.Deps{
		Config:        authConfigSource{runtime: runtimeState},
		Auth:          platformState.Auth,
		LoginFailures: platformState.LoginFailures,
	})
	managementHandler := coreapi.NewHandlers(coreapi.Deps{
		Auth:            platformState.Auth,
		System:          services.System,
		RequestShutdown: deps.RequestShutdown,
	})
	governanceHandler := governanceapi.NewHandlersWithService(services.Governance)
	taskHandler := taskapi.NewHandlers(platformState.Tasks, platformState.TaskExecutor, pluginState.PluginInstaller)
	logHandler := logapi.NewHandlers(services.Logs)
	renderHandler := renderapi.NewHandlers(pluginState.Renderer)
	systemHandler := systemapi.NewHandlers(services.System, platformState.Scheduler)
	protocolHandler := protocolapi.NewHandlers(services.Protocol)
	thirdPartyHandler := thirdpartyapi.NewThirdPartyHandlers(services.ThirdParty, deps.BilibiliAccountClient, services.BilibiliSource, deps.BilibiliHTTPTransport)
	bilibiliHandler := bilibiliapi.NewBilibiliHandlers(services.BilibiliSource, deps.BilibiliQRLogin, deps.BilibiliHTTPTransport)
	eventsWS := managementws.NewEventsHandler(pluginState.Bridge, pluginState.Plugins, services.Protocol, deps.Status, services.GovernanceEvents, services.BilibiliEvents)
	tasksWS := managementws.NewTasksHandler(platformState.Tasks)
	logsWS := managementws.NewLogsHandler(services.Logs)
	consoleWS := managementws.NewConsoleHandler(platformState.Console, pluginState.Plugins)
	pluginManagementUIHandler := pluginui.NewHandlers(pluginui.Deps{
		Plugins:      pluginState.Plugins,
		PluginConfig: pluginState.PluginConfig,
		Secrets:      platformState.Secrets,
		NotifyConfigChange: func(ctx context.Context, pluginID string) {
			dispatch := servicegraph.LocalActionConfigChangedDispatcher(pluginState.Dispatcher)
			if dispatch != nil {
				dispatch(ctx, pluginID)
			}
		},
		RefreshCommands: func(ctx context.Context, pluginID string, settings map[string]any) {
			lifecyclecommands.RefreshPluginCommands(pluginState.Plugins, pluginState.Dispatcher, pluginID, settings)
		},
	})

	router, server := buildAppHTTPServer(serverDeps{
		runtime:            runtimeState,
		auth:               platformState.Auth,
		tasks:              platformState.Tasks,
		plugins:            pluginState.Plugins,
		pluginInstaller:    pluginState.PluginInstaller,
		pluginUninstaller:  pluginState.PluginUninstaller,
		pluginRepository:   pluginState.PluginRepository,
		grantRepository:    pluginState.GrantRepository,
		pluginLifecycle:    services.PluginLifecycle,
		renderer:           pluginState.Renderer,
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
		pluginWebhooks:     services.PluginWebhooks,
		pluginManagementUI: pluginManagementUIHandler,
		metrics:            deps.Metrics,
	})
	return State{
		Router: router,
		Server: server,
		Handlers: Handlers{
			Auth:       authHandler,
			Management: managementHandler,
			Tasks:      taskHandler,
			EventsWS:   eventsWS,
		},
	}
}

type authConfigSource struct {
	runtime RuntimeState
}

func (s authConfigSource) AuthConfig() authapi.Config {
	if s.runtime == nil {
		return authapi.Config{}
	}
	cfg := s.runtime.CurrentConfig()
	return authapi.Config{
		SetupLocalOnly:     cfg.Web.SetupLocalOnly,
		LoginFailureLimit:  authapi.LoginFailureLimit(cfg),
		LoginFailureWindow: authapi.LoginFailureWindow(cfg),
	}
}
