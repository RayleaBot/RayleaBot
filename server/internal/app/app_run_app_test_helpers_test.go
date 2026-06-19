package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app/httpwire"
	"github.com/RayleaBot/RayleaBot/server/internal/app/servicegraph"
	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/eventingress"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/management/configapi"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/management/systemapi"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginconfig "github.com/RayleaBot/RayleaBot/server/internal/plugins/configstore"
	pluginfile "github.com/RayleaBot/RayleaBot/server/internal/plugins/filestore"
	plugingrants "github.com/RayleaBot/RayleaBot/server/internal/plugins/grants"
	pluginkv "github.com/RayleaBot/RayleaBot/server/internal/plugins/kvstore"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	pluginui "github.com/RayleaBot/RayleaBot/server/internal/plugins/managementui"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	runtimeregistry "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/registry"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func newTestAppState(cfg config.Config, logger *slog.Logger) *App {
	if logger == nil {
		logger = slog.Default()
	}
	return &App{
		state: &appRuntimeState{
			Config:    cfg,
			Logger:    logger,
			startedAt: time.Now().UTC(),
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

func testAutoGrantCapabilities(state *appRuntimeState) func() []string {
	return func() []string {
		if state == nil {
			return nil
		}
		return append([]string(nil), state.Config.Permission.AutoGrantCapabilities...)
	}
}

func (a *App) setTestEventIngress(catalog *plugincatalog.Catalog, blacklistRepo permission.BlacklistRepository, sender eventingress.OutboundActionSender, eventBridge *bridge.Bridge) {
	a.setTestEventIngressWithGovernance(catalog, nil, nil, blacklistRepo, sender, eventBridge)
}

func (a *App) setTestEventIngressWithGovernance(catalog *plugincatalog.Catalog, whitelistRepo permission.WhitelistRepository, whitelistState permission.WhitelistStateRepository, blacklistRepo permission.BlacklistRepository, sender eventingress.OutboundActionSender, eventBridge *bridge.Bridge) {
	if a == nil {
		return
	}
	a.pluginStack.Plugins = catalog
	a.pluginStack.WhitelistRepo = whitelistRepo
	a.pluginStack.WhitelistState = whitelistState
	a.pluginStack.BlacklistRepo = blacklistRepo
	a.pluginStack.OutboundSender = sender
	a.pluginStack.Bridge = eventBridge
	menuService := menuext.New(menuext.Deps{
		CurrentConfig: func() config.Config { return a.state.Config },
		Plugins:       catalog,
		Renderer:      a.pluginStack.Renderer,
		Sender:        sender,
		WaitOutbound: func(ctx context.Context, request outbound.MessageLimitRequest) error {
			if a.pluginStack.OutboundLimiter == nil {
				return nil
			}
			return a.pluginStack.OutboundLimiter.Wait(ctx, request)
		},
		Logger: a.state.Logger,
	})
	a.services.EventIngress = eventingress.New(eventingress.Deps{
		CurrentConfig:    a.state.CurrentConfig,
		Logger:           a.state.Logger,
		Plugins:          catalog,
		OutboundSender:   sender,
		OutboundLimiter:  a.pluginStack.OutboundLimiter,
		Renderer:         a.pluginStack.Renderer,
		Menu:             menuService,
		Bridge:           eventBridge,
		MetadataEnricher: a.pluginStack.Adapter,
		WhitelistRepo:    whitelistRepo,
		WhitelistState:   whitelistState,
		BlacklistRepo:    blacklistRepo,
	})
}

func (a *App) setTestLifecycle(catalog *plugincatalog.Catalog, desiredRepo plugins.DesiredStateRepository, grantRepo plugins.GrantRepository, runtimes *runtimeregistry.Registry, dispatcher *dispatch.Dispatcher, pluginConfigRepo pluginconfig.Repository, adapterShell *adaptershell.Shell, webhooks *pluginwebhook.Registry) {
	if a == nil {
		return
	}
	a.pluginStack.Plugins = catalog
	a.pluginStack.PluginRepository = desiredRepo
	a.pluginStack.GrantRepository = grantRepo
	a.runtimes = runtimes
	a.pluginStack.Dispatcher = dispatcher
	a.pluginStack.PluginConfig = pluginConfigRepo
	a.pluginStack.Adapter = adapterShell
	a.pluginStack.Webhooks = webhooks
	a.services.PluginLifecycle = pluginservice.NewController(pluginservice.Deps{
		CurrentConfig:    a.state.CurrentConfig,
		RepoRoot:         a.state.repoRoot,
		Logger:           a.state.Logger,
		Plugins:          catalog,
		DesiredStateRepo: desiredRepo,
		Grants: plugingrants.NewView(plugingrants.ViewDeps{
			Plugins:               catalog,
			GrantRepository:       grantRepo,
			AutoGrantCapabilities: testAutoGrantCapabilities(a.state),
		}),
		Runtimes:     runtimes,
		Dispatcher:   dispatcher,
		Scheduler:    a.platform.Scheduler,
		PluginConfig: pluginConfigRepo,
		Adapter:      adapterShell,
		Webhooks:     webhooks,
		Tasks:        a.platform.Tasks,
	})
}

func (a *App) setTestLocalActions(grantRepo plugins.GrantRepository, pluginConfigRepo pluginconfig.Repository, pluginFiles *pluginfile.Service, pluginKV pluginkv.Repository, schedulerEngine *scheduler.Engine, dispatcher *dispatch.Dispatcher, rendererService *renderservice.Service, adapterShell *adaptershell.Shell, limiter *localaction.PluginLogLimiter, webhookService *pluginwebhook.Service) {
	if a == nil {
		return
	}
	a.pluginStack.GrantRepository = grantRepo
	a.pluginStack.PluginConfig = pluginConfigRepo
	a.pluginStack.PluginFiles = pluginFiles
	a.pluginStack.PluginKV = pluginKV
	a.platform.Scheduler = schedulerEngine
	a.pluginStack.Dispatcher = dispatcher
	a.pluginStack.Renderer = rendererService
	a.pluginStack.Adapter = adapterShell
	a.pluginStack.PluginLogLimiter = limiter
	if a.services.GovernanceEvents == nil {
		a.services.GovernanceEvents = managementevents.NewGovernanceService()
	}
	a.services.Governance = governance.NewService(governance.Deps{
		CurrentConfig:  func() config.Config { return a.state.Config },
		Plugins:        a.pluginStack.Plugins,
		BlacklistRepo:  a.pluginStack.BlacklistRepo,
		WhitelistRepo:  a.pluginStack.WhitelistRepo,
		WhitelistState: a.pluginStack.WhitelistState,
		NotifyChanged:  a.services.GovernanceEvents.PublishChanged,
	})
	a.services.LocalActions = localaction.New(localaction.Deps{
		CurrentConfig: func() config.Config { return a.state.Config },
		Logger:        a.state.Logger,
		RedactText:    a.state.redactString,
		Grants: plugingrants.NewView(plugingrants.ViewDeps{
			Plugins:               a.pluginStack.Plugins,
			GrantRepository:       grantRepo,
			AutoGrantCapabilities: testAutoGrantCapabilities(a.state),
		}),
		PluginConfig:     pluginConfigRepo,
		PluginFiles:      pluginFiles,
		PluginKV:         pluginKV,
		Secrets:          servicegraph.LocalActionSecretReader(a.platform.Secrets),
		Scheduler:        servicegraph.LocalActionScheduler(schedulerEngine),
		Dispatcher:       servicegraph.LocalActionConfigChangedDispatcher(dispatcher),
		Renderer:         servicegraph.LocalActionRenderer(rendererService),
		Adapter:          adapterShell,
		PluginLogLimiter: limiter,
		Governance:       a.services.Governance,
	})
	if webhookService != nil {
		a.services.LocalActions.SetWebhookGateway(webhookService)
	}
}

func (a *App) setTestSystem(taskRegistry *tasks.Registry, taskExecutor *tasks.Executor, rendererService *renderservice.Service, logRepository logging.Repository) {
	if a == nil {
		return
	}
	a.platform.Tasks = taskRegistry
	a.platform.TaskExecutor = taskExecutor
	a.pluginStack.Renderer = rendererService
	a.services.System = systemsvc.New(systemsvc.Deps{
		CurrentConfig:    a.state.CurrentConfig,
		CurrentSummary:   func() config.Summary { return a.state.Summary },
		CurrentRepoRoot:  func() string { return a.state.repoRoot },
		CurrentStartedAt: func() time.Time { return a.state.startedAt },
		Logger:           a.state.Logger,
		Auth:             a.platform.Auth,
		Adapter:          a.pluginStack.Adapter,
		Plugins:          a.pluginStack.Plugins,
		Runtimes:         a.runtimes,
		Renderer:         rendererService,
		PluginRepository: a.pluginStack.PluginRepository,
		TaskExecutor:     taskExecutor,
		LogRepository:    logRepository,
	})
	a.services.System.BindShutdownFlag(&a.process.shuttingDown)
}

func (a *App) setTestWebhookService(secretStore secrets.Store, dispatcher *dispatch.Dispatcher, lifecycle *pluginservice.Controller, registry *pluginwebhook.Registry) {
	if a == nil {
		return
	}
	a.platform.Secrets = secretStore
	a.pluginStack.Dispatcher = dispatcher
	a.pluginStack.Webhooks = registry
	a.services.PluginWebhooks = pluginwebhook.New(pluginwebhook.Deps{
		CurrentConfig: func() config.Config { return a.state.Config },
		Logger:        a.state.Logger,
		Registry:      registry,
		Secrets:       secretStore,
		Plugins:       a.pluginStack.Plugins,
		Dispatcher:    dispatcher,
		Runtime:       lifecycle,
		Grants: plugingrants.NewView(plugingrants.ViewDeps{
			Plugins:               a.pluginStack.Plugins,
			GrantRepository:       a.pluginStack.GrantRepository,
			AutoGrantCapabilities: testAutoGrantCapabilities(a.state),
		}),
	})
	if a.services.LocalActions != nil {
		a.services.LocalActions.SetWebhookGateway(a.services.PluginWebhooks)
	}
}

func (a *App) executeLocalAction(ctx context.Context, pluginID, requestID string, action runtimeaction.Action) (map[string]any, error) {
	return a.services.LocalActions.Execute(ctx, pluginID, requestID, action, runtimeprotocol.Event{})
}

func (a *App) executeOneBotLocalAction(ctx context.Context, pluginID, requestID string, action runtimeaction.Action) (map[string]any, error) {
	return a.services.LocalActions.Execute(ctx, pluginID, requestID, action, runtimeprotocol.Event{})
}

func (a *App) executeLocalActionForEvent(ctx context.Context, pluginID, requestID string, action runtimeaction.Action, parentEvent runtimeprotocol.Event) (map[string]any, error) {
	return a.services.LocalActions.Execute(ctx, pluginID, requestID, action, parentEvent)
}

func (a *App) commandInfoForEvent(event adapterintake.NormalizedEvent) *permission.CommandInfo {
	return a.services.EventIngress.CommandInfoForEvent(event)
}

func (a *App) enrichCommandEvent(event adapterintake.NormalizedEvent) adapterintake.NormalizedEvent {
	return a.services.EventIngress.EnrichCommandEvent(event)
}

func (a *App) handleAdapterEvent(ctx context.Context, event adapterintake.NormalizedEvent) {
	a.services.EventIngress.HandleAdapterEvent(ctx, event)
}

func (a *App) applyChatPolicy(ctx context.Context, event adapterintake.NormalizedEvent) (adapterintake.NormalizedEvent, bool) {
	return a.services.EventIngress.ApplyChatPolicy(ctx, event)
}

func (a *App) autoPrepareRuntimeEnvironments(ctx context.Context) {
	a.services.System.AutoPrepareRuntimeEnvironments(ctx)
}

func (a *App) startupRuntimeState(kind string) (systemsvc.StartupRuntimeState, bool) {
	return a.services.System.StartupRuntimeState(kind)
}

func (a *App) setStartupRuntimeState(kind string, phase systemsvc.StartupRuntimePhase, issue *recovery.CompatibilityIssue) {
	a.services.System.SetStartupRuntimeState(kind, phase, issue)
}

func (a *App) managedRuntimeDiagnostics(pluginsList []plugins.Snapshot) []recovery.CompatibilityIssue {
	return a.services.System.ManagedRuntimeDiagnostics(pluginsList)
}

func (a *App) handleSystemRecoveryRecheck() http.HandlerFunc {
	return systemapi.NewHandlers(a.services.System).HandleSystemRecoveryRecheck()
}

func (a *App) handleSystemRecoveryConfirm() http.HandlerFunc {
	return systemapi.NewHandlers(a.services.System).HandleSystemRecoveryConfirm()
}

func (a *App) handleSystemRuntimeBootstrap() http.HandlerFunc {
	return systemapi.NewHandlers(a.services.System).HandleSystemRuntimeBootstrap()
}

func (a *App) handlePluginWebhook() http.HandlerFunc {
	return a.services.PluginWebhooks.HandleWebhook()
}

func applyConfigApplyEffects(app *App, newCfg config.Config) configapi.ApplyEffects {
	if app == nil {
		return configapi.NewApplyEffects()
	}
	service := httpwire.NewConfigService(httpwire.ConfigDeps{
		Runtime:          app.state,
		Logs:             app.platform.Logs,
		LogRepository:    app.platform.LogRepository,
		Renderer:         app.pluginStack.Renderer,
		PluginLogLimiter: app.pluginStack.PluginLogLimiter,
		OutboundLimiter:  app.pluginStack.OutboundLimiter,
		Protocol:         app.services.Protocol,
		EventIngress:     app.services.EventIngress,
	})
	return configapi.NewHandlers(service).ApplyHotReloadableFields(newCfg)
}

func applyHotReloadableFields(app *App, newCfg config.Config) bool {
	return applyConfigApplyEffects(app, newCfg).RestartRequired()
}

func newPluginWebhookRegistry() *pluginwebhook.Registry {
	return pluginwebhook.NewRegistry()
}

func newPluginLogLimiter(cfg config.Config) *localaction.PluginLogLimiter {
	return localaction.NewPluginLogLimiter(cfg)
}

type stubLifecycleGrantRepository struct {
	grants map[string][]plugins.PluginGrant
}

func (r *stubLifecycleGrantRepository) LoadGrants(_ context.Context, pluginID string) ([]plugins.PluginGrant, error) {
	now := time.Now().UTC()
	var active []plugins.PluginGrant
	for _, grant := range r.grants[pluginID] {
		if grant.ExpiresAt != nil && !grant.ExpiresAt.After(now) {
			continue
		}
		active = append(active, grant)
	}
	return active, nil
}

func (r *stubLifecycleGrantRepository) LoadAllGrants(_ context.Context) (map[string][]string, error) {
	result := make(map[string][]string)
	for pluginID := range r.grants {
		items, _ := r.LoadGrants(context.Background(), pluginID)
		for _, grant := range items {
			result[pluginID] = append(result[pluginID], grant.Capability)
		}
	}
	return result, nil
}

func (r *stubLifecycleGrantRepository) SaveGrant(context.Context, plugins.PluginGrant) error {
	return nil
}

func (r *stubLifecycleGrantRepository) DeleteGrant(context.Context, string, string) error {
	return nil
}

func (r *stubLifecycleGrantRepository) DeleteAllGrants(context.Context, string) error {
	return nil
}

func (a *App) dispatchPluginConfigChanged(ctx context.Context, pluginID string) {
	if a == nil {
		return
	}
	dispatch := servicegraph.LocalActionConfigChangedDispatcher(a.pluginStack.Dispatcher)
	if dispatch != nil {
		dispatch(ctx, pluginID)
	}
}

type pluginManagementUIHTTPDeps struct {
	plugins            *plugincatalog.Catalog
	pluginConfig       pluginconfig.Repository
	secrets            secrets.Store
	notifyConfigChange func(context.Context, string)
	refreshCommands    func(context.Context, string, map[string]any)
}

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
