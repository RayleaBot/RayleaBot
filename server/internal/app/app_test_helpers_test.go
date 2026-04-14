package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginkv"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func newTestAppState(cfg config.Config, logger *slog.Logger) *App {
	if logger == nil {
		logger = slog.Default()
	}
	return &App{
		state: &appRuntimeState{
			Config:               cfg,
			Logger:               logger,
			startedAt:            time.Now().UTC(),
			startupRuntimeStates: newStartupRuntimeStates(nil),
		},
	}
}

func (a *App) setTestEventIngress(catalog *plugins.Catalog, blacklistRepo permission.BlacklistRepository, sender outboundActionSender, eventBridge *bridge.Bridge) {
	if a == nil {
		return
	}
	a.plugins = catalog
	a.blacklistRepo = blacklistRepo
	a.outboundSender = sender
	a.bridge = eventBridge
	a.eventIngress = newEventIngressService(eventIngressDeps{
		state:            a.state,
		plugins:          catalog,
		outboundSender:   sender,
		bridge:           eventBridge,
		metadataEnricher: a.adapter,
		blacklistRepo:    blacklistRepo,
	})
}

func (a *App) setTestLifecycle(catalog *plugins.Catalog, desiredRepo plugins.DesiredStateRepository, grantRepo plugins.GrantRepository, runtimes *runtimeRegistry, dispatcher *dispatch.Dispatcher, pluginConfigRepo pluginconfig.Repository, adapterShell *adapter.Shell, webhooks *pluginWebhookRegistry) {
	if a == nil {
		return
	}
	a.plugins = catalog
	a.pluginRepository = desiredRepo
	a.grantRepository = grantRepo
	a.runtimes = runtimes
	a.dispatcher = dispatcher
	a.pluginConfig = pluginConfigRepo
	a.adapter = adapterShell
	a.webhookRegistry = webhooks
	a.pluginLifecycle = newPluginLifecycleController(pluginLifecycleDeps{
		state:            a.state,
		plugins:          catalog,
		desiredStateRepo: desiredRepo,
		grants: &pluginGrantView{
			state:           a.state,
			plugins:         catalog,
			grantRepository: grantRepo,
		},
		runtimes:     runtimes,
		dispatcher:   dispatcher,
		pluginConfig: pluginConfigRepo,
		adapter:      adapterShell,
		webhooks:     webhooks,
	})
}

func (a *App) setTestLocalActions(grantRepo plugins.GrantRepository, pluginConfigRepo pluginconfig.Repository, pluginFiles *pluginfile.Service, pluginKV pluginkv.Repository, schedulerEngine *scheduler.Engine, dispatcher *dispatch.Dispatcher, rendererService *render.Service, adapterShell *adapter.Shell, limiter *pluginLogLimiter, webhookService *pluginWebhookService) {
	if a == nil {
		return
	}
	a.grantRepository = grantRepo
	a.pluginConfig = pluginConfigRepo
	a.pluginFiles = pluginFiles
	a.pluginKV = pluginKV
	a.scheduler = schedulerEngine
	a.dispatcher = dispatcher
	a.renderer = rendererService
	a.adapter = adapterShell
	a.pluginLogLimiter = limiter
	a.localActions = newLocalActionService(localActionServiceDeps{
		state: a.state,
		grants: &pluginGrantView{
			state:           a.state,
			plugins:         a.plugins,
			grantRepository: grantRepo,
		},
		pluginConfig:     pluginConfigRepo,
		pluginFiles:      pluginFiles,
		pluginKV:         pluginKV,
		scheduler:        schedulerEngine,
		dispatcher:       dispatcher,
		renderer:         rendererService,
		adapter:          adapterShell,
		pluginLogLimiter: limiter,
	})
	if webhookService != nil {
		a.localActions.SetWebhookGateway(webhookService)
	}
}

func (a *App) setTestSystem(taskRegistry *tasks.Registry, taskExecutor *tasks.Executor, rendererService *render.Service, logRepository logging.Repository) {
	if a == nil {
		return
	}
	a.tasks = taskRegistry
	a.taskExecutor = taskExecutor
	a.renderer = rendererService
	a.systemService = newSystemService(systemServiceDeps{
		state:            a.state,
		auth:             a.auth,
		adapter:          a.adapter,
		plugins:          a.plugins,
		runtimes:         a.runtimes,
		renderer:         rendererService,
		pluginRepository: a.pluginRepository,
		taskExecutor:     taskExecutor,
		logRepository:    logRepository,
	})
	a.systemService.shuttingDown = &a.process.shuttingDown
}

func (a *App) setTestWebhookService(secretStore secrets.Store, dispatcher *dispatch.Dispatcher, lifecycle *pluginLifecycleController, registry *pluginWebhookRegistry) {
	if a == nil {
		return
	}
	a.secrets = secretStore
	a.dispatcher = dispatcher
	a.webhookRegistry = registry
	a.pluginWebhooks = newPluginWebhookService(pluginWebhookServiceDeps{
		state:      a.state,
		registry:   registry,
		secrets:    secretStore,
		plugins:    a.plugins,
		dispatcher: dispatcher,
		lifecycle:  lifecycle,
		grants: &pluginGrantView{
			state:           a.state,
			plugins:         a.plugins,
			grantRepository: a.grantRepository,
		},
	})
	if a.localActions != nil {
		a.localActions.SetWebhookGateway(a.pluginWebhooks)
	}
}

func (a *App) executeLocalAction(ctx context.Context, pluginID, requestID string, action runtime.Action) (map[string]any, error) {
	return a.localActions.Execute(ctx, pluginID, requestID, action)
}

func (a *App) executeOneBotLocalAction(ctx context.Context, pluginID, requestID string, action runtime.Action) (map[string]any, error) {
	return a.localActions.executeOneBotLocalAction(ctx, pluginID, requestID, action)
}

func (a *App) commandInfoForEvent(event adapter.NormalizedEvent) *permission.CommandInfo {
	return a.eventIngress.commandInfoForEvent(event)
}

func (a *App) enrichCommandEvent(event adapter.NormalizedEvent) adapter.NormalizedEvent {
	return a.eventIngress.enrichCommandEvent(event)
}

func (a *App) handleAdapterEvent(ctx context.Context, event adapter.NormalizedEvent) {
	a.eventIngress.HandleAdapterEvent(ctx, event)
}

func (a *App) applyChatPolicy(ctx context.Context, event adapter.NormalizedEvent) (adapter.NormalizedEvent, bool) {
	return a.eventIngress.applyChatPolicy(ctx, event)
}

func (a *App) autoPrepareRuntimeEnvironments(ctx context.Context) {
	a.systemService.autoPrepareRuntimeEnvironments(ctx)
}

func (a *App) startupRuntimeState(kind string) (startupRuntimeState, bool) {
	return a.systemService.startupRuntimeState(kind)
}

func (a *App) setStartupRuntimeState(kind string, phase startupRuntimePhase, issue *recovery.CompatibilityIssue) {
	a.systemService.setStartupRuntimeState(kind, phase, issue)
}

func (a *App) managedRuntimeDiagnostics(pluginsList []plugins.Snapshot) []recovery.CompatibilityIssue {
	return a.systemService.managedRuntimeDiagnostics(pluginsList)
}

func (a *App) handleSystemRecoveryRecheck() http.HandlerFunc {
	return newSystemHTTPHandlers(a.systemService).handleSystemRecoveryRecheck()
}

func (a *App) handleSystemRecoveryConfirm() http.HandlerFunc {
	return newSystemHTTPHandlers(a.systemService).handleSystemRecoveryConfirm()
}

func (a *App) handleSystemRuntimeBootstrap() http.HandlerFunc {
	return newSystemHTTPHandlers(a.systemService).handleSystemRuntimeBootstrap()
}

func (a *App) handlePluginWebhook() http.HandlerFunc {
	return newPluginWebhookHTTPHandlers(a.pluginWebhooks).handlePluginWebhook()
}

func applyHotReloadableFields(app *App, newCfg config.Config) bool {
	if app == nil {
		return false
	}
	return newConfigHTTPHandlers(configHTTPDeps{
		state:            app.state,
		logs:             app.logs,
		logRepository:    app.logRepository,
		renderer:         app.renderer,
		pluginLogLimiter: app.pluginLogLimiter,
		protocol:         app.protocol,
		eventIngress:     app.eventIngress,
		blacklistRepo:    app.blacklistRepo,
	}).applyHotReloadableFields(newCfg)
}
