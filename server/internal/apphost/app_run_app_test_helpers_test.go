package apphost

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/adapter/intake"
	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/adapter/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/eventingress"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/localaction"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	managementhttp "github.com/RayleaBot/RayleaBot/server/internal/management/http"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginconfig"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginkv"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/service"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginui"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginwebhook"
	"github.com/RayleaBot/RayleaBot/server/internal/recovery"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/runtime/action"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/runtime/protocol"
	runtimeregistry "github.com/RayleaBot/RayleaBot/server/internal/runtime/registry"
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

func (a *App) setTestEventIngress(catalog *plugincatalog.Catalog, blacklistRepo permission.BlacklistRepository, sender eventingress.OutboundActionSender, eventBridge *bridge.Bridge) {
	a.setTestEventIngressWithGovernance(catalog, nil, nil, blacklistRepo, sender, eventBridge)
}

func (a *App) setTestEventIngressWithGovernance(catalog *plugincatalog.Catalog, whitelistRepo permission.WhitelistRepository, whitelistState permission.WhitelistStateRepository, blacklistRepo permission.BlacklistRepository, sender eventingress.OutboundActionSender, eventBridge *bridge.Bridge) {
	if a == nil {
		return
	}
	a.pluginStack.Plugins = catalog
	a.pluginStack.whitelistRepo = whitelistRepo
	a.pluginStack.whitelistState = whitelistState
	a.pluginStack.blacklistRepo = blacklistRepo
	a.pluginStack.outboundSender = sender
	a.pluginStack.Bridge = eventBridge
	menuService := menuext.New(menuext.Deps{
		CurrentConfig: func() config.Config { return a.state.Config },
		Plugins:       catalog,
		Renderer:      a.pluginStack.renderer,
		Sender:        sender,
		WaitOutbound: func(ctx context.Context, request outbound.MessageLimitRequest) error {
			if a.pluginStack.outboundLimiter == nil {
				return nil
			}
			return a.pluginStack.outboundLimiter.Wait(ctx, request)
		},
		Logger: a.state.Logger,
	})
	a.services.eventIngress = eventingress.New(eventingress.Deps{
		CurrentConfig:    a.state.CurrentConfig,
		Logger:           a.state.Logger,
		Plugins:          catalog,
		OutboundSender:   sender,
		OutboundLimiter:  a.pluginStack.outboundLimiter,
		Renderer:         a.pluginStack.renderer,
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
	a.pluginStack.pluginRepository = desiredRepo
	a.pluginStack.grantRepository = grantRepo
	a.runtimes = runtimes
	a.pluginStack.Dispatcher = dispatcher
	a.pluginStack.pluginConfig = pluginConfigRepo
	a.pluginStack.Adapter = adapterShell
	a.pluginStack.webhooks = webhooks
	a.services.pluginLifecycle = pluginservice.NewController(pluginservice.Deps{
		CurrentConfig:    a.state.CurrentConfig,
		RepoRoot:         a.state.repoRoot,
		Logger:           a.state.Logger,
		Plugins:          catalog,
		DesiredStateRepo: desiredRepo,
		Grants: pluginservice.NewGrantView(pluginservice.GrantViewDeps{
			Plugins:               catalog,
			GrantRepository:       grantRepo,
			AutoGrantCapabilities: currentPluginAutoGrantCapabilities(a.state),
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
	a.pluginStack.grantRepository = grantRepo
	a.pluginStack.pluginConfig = pluginConfigRepo
	a.pluginStack.pluginFiles = pluginFiles
	a.pluginStack.pluginKV = pluginKV
	a.platform.Scheduler = schedulerEngine
	a.pluginStack.Dispatcher = dispatcher
	a.pluginStack.renderer = rendererService
	a.pluginStack.Adapter = adapterShell
	a.pluginStack.pluginLogLimiter = limiter
	if a.services.governanceEvents == nil {
		a.services.governanceEvents = managementevents.NewGovernanceService()
	}
	a.services.governance = governance.NewService(governance.Deps{
		CurrentConfig:  func() config.Config { return a.state.Config },
		Plugins:        a.pluginStack.Plugins,
		BlacklistRepo:  a.pluginStack.blacklistRepo,
		WhitelistRepo:  a.pluginStack.whitelistRepo,
		WhitelistState: a.pluginStack.whitelistState,
		NotifyChanged:  a.services.governanceEvents.PublishChanged,
	})
	a.services.localActions = localaction.New(localaction.Deps{
		CurrentConfig: func() config.Config { return a.state.Config },
		Logger:        a.state.Logger,
		RedactText:    a.state.redactString,
		Grants: pluginservice.NewGrantView(pluginservice.GrantViewDeps{
			Plugins:               a.pluginStack.Plugins,
			GrantRepository:       grantRepo,
			AutoGrantCapabilities: currentPluginAutoGrantCapabilities(a.state),
		}),
		PluginConfig:     pluginConfigRepo,
		PluginFiles:      pluginFiles,
		PluginKV:         pluginKV,
		Secrets:          a.platform.Secrets,
		Scheduler:        schedulerEngine,
		Dispatcher:       dispatcher,
		Renderer:         rendererService,
		Adapter:          adapterShell,
		PluginLogLimiter: limiter,
		Governance:       a.services.governance,
		ThirdParty:       a.services.thirdParty,
	})
	if webhookService != nil {
		a.services.localActions.SetWebhookGateway(webhookService)
	}
}

func (a *App) setTestSystem(taskRegistry *tasks.Registry, taskExecutor *tasks.Executor, rendererService *renderservice.Service, logRepository logging.Repository) {
	if a == nil {
		return
	}
	a.platform.Tasks = taskRegistry
	a.platform.taskExecutor = taskExecutor
	a.pluginStack.renderer = rendererService
	a.services.system = systemsvc.New(systemsvc.Deps{
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
		PluginRepository: a.pluginStack.pluginRepository,
		TaskExecutor:     taskExecutor,
		LogRepository:    logRepository,
	})
	a.services.system.BindShutdownFlag(&a.process.shuttingDown)
}

func (a *App) setTestWebhookService(secretStore secrets.Store, dispatcher *dispatch.Dispatcher, lifecycle *pluginservice.Controller, registry *pluginwebhook.Registry) {
	if a == nil {
		return
	}
	a.platform.Secrets = secretStore
	a.pluginStack.Dispatcher = dispatcher
	a.pluginStack.webhooks = registry
	a.services.pluginWebhooks = pluginwebhook.New(pluginwebhook.Deps{
		CurrentConfig: func() config.Config { return a.state.Config },
		Logger:        a.state.Logger,
		Registry:      registry,
		Secrets:       secretStore,
		Plugins:       a.pluginStack.Plugins,
		Dispatcher:    dispatcher,
		Runtime:       lifecycle,
		Grants: pluginservice.NewGrantView(pluginservice.GrantViewDeps{
			Plugins:               a.pluginStack.Plugins,
			GrantRepository:       a.pluginStack.grantRepository,
			AutoGrantCapabilities: currentPluginAutoGrantCapabilities(a.state),
		}),
	})
	if a.services.localActions != nil {
		a.services.localActions.SetWebhookGateway(a.services.pluginWebhooks)
	}
}

func (a *App) executeLocalAction(ctx context.Context, pluginID, requestID string, action runtimeaction.Action) (map[string]any, error) {
	return a.services.localActions.Execute(ctx, pluginID, requestID, action, runtimeprotocol.Event{})
}

func (a *App) executeOneBotLocalAction(ctx context.Context, pluginID, requestID string, action runtimeaction.Action) (map[string]any, error) {
	return a.services.localActions.Execute(ctx, pluginID, requestID, action, runtimeprotocol.Event{})
}

func (a *App) executeLocalActionForEvent(ctx context.Context, pluginID, requestID string, action runtimeaction.Action, parentEvent runtimeprotocol.Event) (map[string]any, error) {
	return a.services.localActions.Execute(ctx, pluginID, requestID, action, parentEvent)
}

func (a *App) commandInfoForEvent(event adapterintake.NormalizedEvent) *permission.CommandInfo {
	return a.services.eventIngress.CommandInfoForEvent(event)
}

func (a *App) enrichCommandEvent(event adapterintake.NormalizedEvent) adapterintake.NormalizedEvent {
	return a.services.eventIngress.EnrichCommandEvent(event)
}

func (a *App) handleAdapterEvent(ctx context.Context, event adapterintake.NormalizedEvent) {
	a.services.eventIngress.HandleAdapterEvent(ctx, event)
}

func (a *App) applyChatPolicy(ctx context.Context, event adapterintake.NormalizedEvent) (adapterintake.NormalizedEvent, bool) {
	return a.services.eventIngress.ApplyChatPolicy(ctx, event)
}

func (a *App) autoPrepareRuntimeEnvironments(ctx context.Context) {
	a.services.system.AutoPrepareRuntimeEnvironments(ctx)
}

func (a *App) startupRuntimeState(kind string) (systemsvc.StartupRuntimeState, bool) {
	return a.services.system.StartupRuntimeState(kind)
}

func (a *App) setStartupRuntimeState(kind string, phase systemsvc.StartupRuntimePhase, issue *recovery.CompatibilityIssue) {
	a.services.system.SetStartupRuntimeState(kind, phase, issue)
}

func (a *App) managedRuntimeDiagnostics(pluginsList []plugins.Snapshot) []recovery.CompatibilityIssue {
	return a.services.system.ManagedRuntimeDiagnostics(pluginsList)
}

func (a *App) handleSystemRecoveryRecheck() http.HandlerFunc {
	return managementhttp.NewSystemHandlers(a.services.system).HandleSystemRecoveryRecheck()
}

func (a *App) handleSystemRecoveryConfirm() http.HandlerFunc {
	return managementhttp.NewSystemHandlers(a.services.system).HandleSystemRecoveryConfirm()
}

func (a *App) handleSystemRuntimeBootstrap() http.HandlerFunc {
	return managementhttp.NewSystemHandlers(a.services.system).HandleSystemRuntimeBootstrap()
}

func (a *App) handlePluginWebhook() http.HandlerFunc {
	return a.services.pluginWebhooks.HandleWebhook()
}

func applyConfigApplyEffects(app *App, newCfg config.Config) managementhttp.ConfigApplyEffects {
	if app == nil {
		return managementhttp.NewConfigApplyEffects()
	}
	service := newConfigHTTPService(configHTTPDeps{
		state:            app.state,
		logs:             app.platform.Logs,
		logRepository:    app.platform.LogRepository,
		renderer:         app.pluginStack.renderer,
		pluginLogLimiter: app.pluginStack.pluginLogLimiter,
		outboundLimiter:  app.pluginStack.outboundLimiter,
		protocol:         app.services.protocol,
		eventIngress:     app.services.eventIngress,
		blacklistRepo:    app.pluginStack.blacklistRepo,
	})
	return managementhttp.NewConfigHandlers(service).ApplyHotReloadableFields(newCfg)
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
	if a == nil || a.services.localActions == nil {
		return
	}
	a.services.localActions.DispatchPluginConfigChanged(ctx, pluginID)
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
