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
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginkv"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginui"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwebhook"
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

func defaultAdapterTestConfig() config.AdapterConfig {
	return config.AdapterConfig{
		ConnectTimeoutSeconds:   15,
		ReconnectInitialSeconds: 2,
		ReconnectMultiplier:     2,
		ReconnectMaxSeconds:     120,
		ReconnectJitterRatio:    0.2,
	}
}

func (a *App) setTestEventIngress(catalog *plugins.Catalog, blacklistRepo permission.BlacklistRepository, sender outboundActionSender, eventBridge *bridge.Bridge) {
	a.setTestEventIngressWithGovernance(catalog, nil, nil, blacklistRepo, sender, eventBridge)
}

func (a *App) setTestEventIngressWithGovernance(catalog *plugins.Catalog, whitelistRepo permission.WhitelistRepository, whitelistState permission.WhitelistStateRepository, blacklistRepo permission.BlacklistRepository, sender outboundActionSender, eventBridge *bridge.Bridge) {
	if a == nil {
		return
	}
	a.plugins = catalog
	a.whitelistRepo = whitelistRepo
	a.whitelistState = whitelistState
	a.blacklistRepo = blacklistRepo
	a.outboundSender = sender
	a.bridge = eventBridge
	menuService := menuext.New(menuext.Deps{
		CurrentConfig: func() config.Config { return a.state.Config },
		Plugins:       catalog,
		Renderer:      a.renderer,
		Sender:        sender,
		WaitOutbound: func(ctx context.Context, request outbound.MessageLimitRequest) error {
			if a.outboundLimiter == nil {
				return nil
			}
			return a.outboundLimiter.Wait(ctx, request)
		},
		Logger: a.state.Logger,
	})
	a.eventIngress = newEventIngressService(eventIngressDeps{
		state:            a.state,
		plugins:          catalog,
		outboundSender:   sender,
		outboundLimiter:  a.outboundLimiter,
		renderer:         a.renderer,
		menu:             menuService,
		bridge:           eventBridge,
		metadataEnricher: a.adapter,
		whitelistRepo:    whitelistRepo,
		whitelistState:   whitelistState,
		blacklistRepo:    blacklistRepo,
	})
}

func (a *App) setTestLifecycle(catalog *plugins.Catalog, desiredRepo plugins.DesiredStateRepository, grantRepo plugins.GrantRepository, runtimes *runtimeRegistry, dispatcher *dispatch.Dispatcher, pluginConfigRepo pluginconfig.Repository, adapterShell *adapter.Shell, webhooks *pluginwebhook.Registry) {
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
		scheduler:    a.scheduler,
		pluginConfig: pluginConfigRepo,
		adapter:      adapterShell,
		webhooks:     webhooks,
		tasks:        a.tasks,
	})
}

func (a *App) setTestLocalActions(grantRepo plugins.GrantRepository, pluginConfigRepo pluginconfig.Repository, pluginFiles *pluginfile.Service, pluginKV pluginkv.Repository, schedulerEngine *scheduler.Engine, dispatcher *dispatch.Dispatcher, rendererService *render.Service, adapterShell *adapter.Shell, limiter *localaction.PluginLogLimiter, webhookService *pluginwebhook.Service) {
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
	if a.governanceEvents == nil {
		a.governanceEvents = newGovernanceEventService()
	}
	a.governance = governance.NewService(governance.Deps{
		CurrentConfig:  func() config.Config { return a.state.Config },
		Plugins:        a.plugins,
		BlacklistRepo:  a.blacklistRepo,
		WhitelistRepo:  a.whitelistRepo,
		WhitelistState: a.whitelistState,
		NotifyChanged:  a.governanceEvents.PublishChanged,
	})
	a.localActions = localaction.New(localaction.Deps{
		CurrentConfig: func() config.Config { return a.state.Config },
		Logger:        a.state.Logger,
		RedactText:    a.state.redactString,
		Grants: &pluginGrantView{
			state:           a.state,
			plugins:         a.plugins,
			grantRepository: grantRepo,
		},
		PluginConfig:     pluginConfigRepo,
		PluginFiles:      pluginFiles,
		PluginKV:         pluginKV,
		Secrets:          a.secrets,
		Scheduler:        schedulerEngine,
		Dispatcher:       dispatcher,
		Renderer:         rendererService,
		Adapter:          adapterShell,
		PluginLogLimiter: limiter,
		Governance:       a.governance,
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

func (a *App) setTestWebhookService(secretStore secrets.Store, dispatcher *dispatch.Dispatcher, lifecycle *pluginLifecycleController, registry *pluginwebhook.Registry) {
	if a == nil {
		return
	}
	a.secrets = secretStore
	a.dispatcher = dispatcher
	a.webhookRegistry = registry
	a.pluginWebhooks = pluginwebhook.New(pluginwebhook.Deps{
		CurrentConfig: func() config.Config { return a.state.Config },
		Logger:        a.state.Logger,
		Registry:      registry,
		Secrets:       secretStore,
		Plugins:       a.plugins,
		Dispatcher:    dispatcher,
		Runtime:       lifecycle,
		Grants: &pluginGrantView{
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
	return a.localActions.Execute(ctx, pluginID, requestID, action, runtime.Event{})
}

func (a *App) executeOneBotLocalAction(ctx context.Context, pluginID, requestID string, action runtime.Action) (map[string]any, error) {
	return a.localActions.Execute(ctx, pluginID, requestID, action, runtime.Event{})
}

func (a *App) executeLocalActionForEvent(ctx context.Context, pluginID, requestID string, action runtime.Action, parentEvent runtime.Event) (map[string]any, error) {
	return a.localActions.Execute(ctx, pluginID, requestID, action, parentEvent)
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
	return a.pluginWebhooks.HandleWebhook()
}

func applyConfigApplyEffects(app *App, newCfg config.Config) configApplyEffects {
	if app == nil {
		return newConfigApplyEffects()
	}
	return newConfigHTTPHandlers(configHTTPDeps{
		state:            app.state,
		logs:             app.logs,
		logRepository:    app.logRepository,
		renderer:         app.renderer,
		pluginLogLimiter: app.pluginLogLimiter,
		outboundLimiter:  app.outboundLimiter,
		protocol:         app.protocol,
		eventIngress:     app.eventIngress,
		blacklistRepo:    app.blacklistRepo,
	}).applyHotReloadableFields(newCfg)
}

func applyHotReloadableFields(app *App, newCfg config.Config) bool {
	return applyConfigApplyEffects(app, newCfg).restartRequired()
}

func newPluginWebhookRegistry() *pluginwebhook.Registry {
	return pluginwebhook.NewRegistry()
}

func newPluginLogLimiter(cfg config.Config) *localaction.PluginLogLimiter {
	return localaction.NewPluginLogLimiter(cfg)
}

func (a *App) dispatchPluginConfigChanged(ctx context.Context, pluginID string) {
	if a == nil || a.localActions == nil {
		return
	}
	a.localActions.DispatchPluginConfigChanged(ctx, pluginID)
}

type pluginManagementUIHTTPDeps struct {
	plugins            *plugins.Catalog
	pluginConfig       pluginconfig.Repository
	secrets            secrets.Store
	notifyConfigChange func(context.Context, string)
	refreshCommands    func(context.Context, string, map[string]any)
}

type pluginSettingsResponse = pluginui.PluginSettingsResponse
type pluginSettingsUpdateResponse = pluginui.PluginSettingsUpdateResponse

type pluginManagementUIHTTPHandlers struct {
	*pluginui.Handlers
}

func newPluginManagementUIHTTPHandlers(deps pluginManagementUIHTTPDeps) *pluginManagementUIHTTPHandlers {
	return &pluginManagementUIHTTPHandlers{Handlers: pluginui.NewHandlers(pluginui.Deps{
		Plugins:            deps.plugins,
		PluginConfig:       deps.pluginConfig,
		Secrets:            deps.secrets,
		NotifyConfigChange: deps.notifyConfigChange,
		RefreshCommands:    deps.refreshCommands,
	})}
}

func (h *pluginManagementUIHTTPHandlers) handlePluginManagementUIStatic() http.HandlerFunc {
	return h.Handlers.HandlePluginManagementUIStatic()
}

func (h *pluginManagementUIHTTPHandlers) handlePluginSettingsGet() http.HandlerFunc {
	return h.Handlers.HandlePluginSettingsGet()
}

func (h *pluginManagementUIHTTPHandlers) handlePluginSettingsPut() http.HandlerFunc {
	return h.Handlers.HandlePluginSettingsPut()
}

func (h *pluginManagementUIHTTPHandlers) handlePluginSecretsGet() http.HandlerFunc {
	return h.Handlers.HandlePluginSecretsGet()
}

func (h *pluginManagementUIHTTPHandlers) handlePluginSecretsPut() http.HandlerFunc {
	return h.Handlers.HandlePluginSecretsPut()
}
