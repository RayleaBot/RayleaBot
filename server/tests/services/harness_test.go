package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	"github.com/RayleaBot/RayleaBot/server/internal/app/httpwire"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/app/renderstack"
	"github.com/RayleaBot/RayleaBot/server/internal/app/servicegraph"
	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	adaptershell "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/shell"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/eventingress"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/management/configapi"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/management/systemapi"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	defaultactionmodules "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/defaultmodules"
	plugincapabilityview "github.com/RayleaBot/RayleaBot/server/internal/plugins/capabilityview"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginconfig "github.com/RayleaBot/RayleaBot/server/internal/plugins/configstore"
	pluginfile "github.com/RayleaBot/RayleaBot/server/internal/plugins/filestore"
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

// serviceHarness assembles individual application services in isolation, the
// same way the composition root does, but without building a full *app.App. It
// lets service-level tests construct exactly the collaborators they exercise.
type serviceHarness struct {
	state        *harnessState
	process      harnessProcess
	platform     appplatform.State
	pluginStack  pluginstack.State
	renderStack  renderstack.State
	eventStack   eventstack.State
	services     servicegraph.Services
	runtimes     *runtimeregistry.Registry
	capabilities localaction.CapabilityView

	blacklistRepo  permission.BlacklistRepository
	whitelistRepo  permission.WhitelistRepository
	whitelistState permission.WhitelistStateRepository
}

type harnessProcess struct {
	shuttingDown atomic.Bool
}

// harnessState mirrors the app runtime state and satisfies both the
// httpwire.RuntimeState and servicegraph.RuntimeState interfaces.
type harnessState struct {
	Config             config.Config
	Summary            config.Summary
	Logger             *slog.Logger
	LogLevel           *logging.LevelController
	repoRoot           string
	redactText         func(string) string
	addRedactionValues func(...string)
	startedAt          time.Time
}

func (s *harnessState) CurrentConfig() config.Config {
	if s == nil {
		return config.Config{}
	}
	return s.Config
}

func (s *harnessState) CurrentSummary() config.Summary {
	if s == nil {
		return config.Summary{}
	}
	return s.Summary
}

func (s *harnessState) SetConfig(cfg config.Config) {
	if s != nil {
		s.Config = cfg
	}
}

func (s *harnessState) SetSummary(summary config.Summary) {
	if s != nil {
		s.Summary = summary
	}
}

func (s *harnessState) RuntimeLogger() *slog.Logger {
	if s == nil {
		return nil
	}
	return s.Logger
}

func (s *harnessState) RuntimeLogLevel() *logging.LevelController {
	if s == nil {
		return nil
	}
	return s.LogLevel
}

func (s *harnessState) RepoRoot() string {
	if s == nil {
		return ""
	}
	return s.repoRoot
}

func (s *harnessState) StartedAt() time.Time {
	if s == nil {
		return time.Time{}
	}
	return s.startedAt
}

func (s *harnessState) RedactString(value string) string {
	return s.redactString(value)
}

func (s *harnessState) AddRedactionValues(values ...string) {
	if s == nil || s.addRedactionValues == nil {
		return
	}
	s.addRedactionValues(values...)
}

func (s *harnessState) redactString(value string) string {
	if s == nil || s.redactText == nil {
		return value
	}
	return s.redactText(value)
}

func newTestAppState(cfg config.Config, logger *slog.Logger) *serviceHarness {
	if logger == nil {
		logger = slog.Default()
	}
	return &serviceHarness{
		state: &harnessState{
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

func (a *serviceHarness) setTestEventIngress(catalog *plugincatalog.Catalog, blacklistRepo permission.BlacklistRepository, sender eventingress.OutboundActionSender, eventBridge *bridge.Bridge) {
	a.setTestEventIngressWithGovernance(catalog, nil, nil, blacklistRepo, sender, eventBridge)
}

func (a *serviceHarness) setTestEventIngressWithGovernance(catalog *plugincatalog.Catalog, whitelistRepo permission.WhitelistRepository, whitelistState permission.WhitelistStateRepository, blacklistRepo permission.BlacklistRepository, sender eventingress.OutboundActionSender, eventBridge *bridge.Bridge) {
	if a == nil {
		return
	}
	a.pluginStack.Plugins = catalog
	a.whitelistRepo = whitelistRepo
	a.whitelistState = whitelistState
	a.blacklistRepo = blacklistRepo
	a.eventStack.OutboundSender = sender
	a.eventStack.Bridge = eventBridge
	menuService := menuext.New(menuext.Deps{
		CurrentConfig: func() config.Config { return a.state.Config },
		Plugins:       catalog,
		Renderer:      a.renderStack.Renderer,
		Sender:        sender,
		WaitOutbound: func(ctx context.Context, request outbound.MessageLimitRequest) error {
			if a.eventStack.OutboundLimiter == nil {
				return nil
			}
			return a.eventStack.OutboundLimiter.Wait(ctx, request)
		},
		Logger: a.state.Logger,
	})
	a.services.EventIngress = eventingress.New(eventingress.Deps{
		CurrentConfig:    a.state.CurrentConfig,
		Logger:           a.state.Logger,
		Plugins:          catalog,
		OutboundSender:   sender,
		OutboundLimiter:  a.eventStack.OutboundLimiter,
		Renderer:         a.renderStack.Renderer,
		Menu:             menuService,
		Bridge:           eventBridge,
		MetadataEnricher: a.eventStack.Adapter,
		WhitelistRepo:    whitelistRepo,
		WhitelistState:   whitelistState,
		BlacklistRepo:    blacklistRepo,
	})
}

func (a *serviceHarness) setTestLifecycle(catalog *plugincatalog.Catalog, desiredRepo plugins.DesiredStateRepository, _ any, runtimes *runtimeregistry.Registry, dispatcher *dispatch.Dispatcher, pluginConfigRepo pluginconfig.Repository, adapterShell *adaptershell.Shell, webhooks *pluginwebhook.Registry) {
	if a == nil {
		return
	}
	a.pluginStack.Plugins = catalog
	a.pluginStack.PluginRepository = desiredRepo
	a.runtimes = runtimes
	a.eventStack.Dispatcher = dispatcher
	a.pluginStack.PluginConfig = pluginConfigRepo
	a.eventStack.Adapter = adapterShell
	a.pluginStack.Webhooks = webhooks
	a.services.PluginLifecycle = pluginservice.NewController(pluginservice.Deps{
		CurrentConfig:    a.state.CurrentConfig,
		RepoRoot:         a.state.repoRoot,
		Logger:           a.state.Logger,
		Plugins:          catalog,
		DesiredStateRepo: desiredRepo,
		Runtimes:         runtimes,
		Dispatcher:       dispatcher,
		Scheduler:        a.platform.Scheduler,
		PluginConfig:     pluginConfigRepo,
		Adapter:          adapterShell,
		Webhooks:         webhooks,
		Tasks:            a.platform.Tasks,
	})
}

func (a *serviceHarness) setTestLocalActions(capabilities localaction.CapabilityView, pluginConfigRepo pluginconfig.Repository, pluginFiles *pluginfile.Service, pluginKV pluginkv.Repository, schedulerEngine *scheduler.Engine, dispatcher *dispatch.Dispatcher, rendererService *renderservice.Service, adapterShell *adaptershell.Shell, limiter *localaction.PluginLogLimiter, webhookService *pluginwebhook.Service) {
	if a == nil {
		return
	}
	a.capabilities = capabilities
	if a.capabilities == nil {
		a.capabilities = a.currentCapabilityView()
	}
	a.pluginStack.PluginConfig = pluginConfigRepo
	a.pluginStack.PluginFiles = pluginFiles
	a.pluginStack.PluginKV = pluginKV
	a.platform.Scheduler = schedulerEngine
	a.eventStack.Dispatcher = dispatcher
	a.renderStack.Renderer = rendererService
	a.eventStack.Adapter = adapterShell
	a.pluginStack.PluginLogLimiter = limiter
	if a.services.GovernanceEvents == nil {
		a.services.GovernanceEvents = managementevents.NewGovernanceService()
	}
	a.services.Governance = governance.NewService(governance.Deps{
		CurrentConfig:  func() config.Config { return a.state.Config },
		Plugins:        a.pluginStack.Plugins,
		BlacklistRepo:  a.blacklistRepo,
		WhitelistRepo:  a.whitelistRepo,
		WhitelistState: a.whitelistState,
		NotifyChanged:  a.services.GovernanceEvents.PublishChanged,
	})
	a.services.LocalActions = localaction.New(localaction.Deps{
		CurrentConfig:    func() config.Config { return a.state.Config },
		Logger:           a.state.Logger,
		RedactText:       a.state.redactString,
		Capabilities:     a.capabilities,
		PluginConfig:     pluginConfigRepo,
		PluginFiles:      pluginFiles,
		PluginKV:         pluginKV,
		Secrets:          localaction.SecretReaderFromStore(a.platform.Secrets),
		Scheduler:        localaction.Scheduler(schedulerEngine),
		Dispatcher:       localaction.ConfigChangedDispatcher(dispatcher),
		Renderer:         localaction.RendererFromService(rendererService),
		Adapter:          adapterShell,
		PluginLogLimiter: limiter,
		Governance:       a.services.Governance,
		Registrars:       defaultactionmodules.Registrars(),
	})
	if webhookService != nil {
		a.services.LocalActions.SetWebhookGateway(webhookService)
	}
}

func (a *serviceHarness) setTestSystem(taskRegistry *tasks.Registry, taskExecutor *tasks.Executor, rendererService *renderservice.Service, logRepository logging.Repository) {
	if a == nil {
		return
	}
	a.platform.Tasks = taskRegistry
	a.platform.TaskExecutor = taskExecutor
	a.renderStack.Renderer = rendererService
	a.services.System = systemsvc.New(systemsvc.Deps{
		CurrentConfig:    a.state.CurrentConfig,
		CurrentSummary:   func() config.Summary { return a.state.Summary },
		CurrentRepoRoot:  func() string { return a.state.repoRoot },
		CurrentStartedAt: func() time.Time { return a.state.startedAt },
		Logger:           a.state.Logger,
		Auth:             a.platform.Auth,
		Adapter:          a.eventStack.Adapter,
		Plugins:          a.pluginStack.Plugins,
		Runtimes:         a.runtimes,
		Renderer:         rendererService,
		PluginRepository: a.pluginStack.PluginRepository,
		TaskExecutor:     taskExecutor,
		LogRepository:    logRepository,
	})
	a.services.System.BindShutdownFlag(&a.process.shuttingDown)
}

func (a *serviceHarness) setTestWebhookService(secretStore secrets.Store, dispatcher *dispatch.Dispatcher, lifecycle *pluginservice.Controller, registry *pluginwebhook.Registry) {
	if a == nil {
		return
	}
	a.platform.Secrets = secretStore
	a.eventStack.Dispatcher = dispatcher
	a.pluginStack.Webhooks = registry
	a.services.PluginWebhooks = pluginwebhook.New(pluginwebhook.Deps{
		CurrentConfig: func() config.Config { return a.state.Config },
		Logger:        a.state.Logger,
		Registry:      registry,
		Secrets:       secretStore,
		Plugins:       a.pluginStack.Plugins,
		Dispatcher:    dispatcher,
		Runtime:       lifecycle,
		Capabilities:  a.currentCapabilityView(),
	})
	if a.services.LocalActions != nil {
		a.services.LocalActions.SetWebhookGateway(a.services.PluginWebhooks)
	}
}

func (a *serviceHarness) executeLocalAction(ctx context.Context, pluginID, requestID string, action runtimeaction.Action) (map[string]any, error) {
	return a.services.LocalActions.Execute(ctx, pluginID, requestID, action, runtimeprotocol.Event{})
}

func (a *serviceHarness) executeOneBotLocalAction(ctx context.Context, pluginID, requestID string, action runtimeaction.Action) (map[string]any, error) {
	return a.services.LocalActions.Execute(ctx, pluginID, requestID, action, runtimeprotocol.Event{})
}

func (a *serviceHarness) executeLocalActionForEvent(ctx context.Context, pluginID, requestID string, action runtimeaction.Action, parentEvent runtimeprotocol.Event) (map[string]any, error) {
	return a.services.LocalActions.Execute(ctx, pluginID, requestID, action, parentEvent)
}

func (a *serviceHarness) commandInfoForEvent(event adapterintake.NormalizedEvent) *permission.CommandInfo {
	return a.services.EventIngress.CommandInfoForEvent(event)
}

func (a *serviceHarness) enrichCommandEvent(event adapterintake.NormalizedEvent) adapterintake.NormalizedEvent {
	return a.services.EventIngress.EnrichCommandEvent(event)
}

func (a *serviceHarness) handleAdapterEvent(ctx context.Context, event adapterintake.NormalizedEvent) {
	a.services.EventIngress.HandleAdapterEvent(ctx, event)
}

func (a *serviceHarness) applyChatPolicy(ctx context.Context, event adapterintake.NormalizedEvent) (adapterintake.NormalizedEvent, bool) {
	return a.services.EventIngress.ApplyChatPolicy(ctx, event)
}

func (a *serviceHarness) autoPrepareRuntimeEnvironments(ctx context.Context) {
	a.services.System.AutoPrepareRuntimeEnvironments(ctx)
}

func (a *serviceHarness) startupRuntimeState(kind string) (systemsvc.StartupRuntimeState, bool) {
	return a.services.System.StartupRuntimeState(kind)
}

func (a *serviceHarness) setStartupRuntimeState(kind string, phase systemsvc.StartupRuntimePhase, issue *recovery.CompatibilityIssue) {
	a.services.System.SetStartupRuntimeState(kind, phase, issue)
}

func (a *serviceHarness) managedRuntimeDiagnostics(pluginsList []plugins.Snapshot) []recovery.CompatibilityIssue {
	return a.services.System.ManagedRuntimeDiagnostics(pluginsList)
}

func (a *serviceHarness) handleSystemRecoveryRecheck() http.HandlerFunc {
	return systemapi.NewHandlers(a.services.System).HandleSystemRecoveryRecheck()
}

func (a *serviceHarness) handleSystemRecoveryConfirm() http.HandlerFunc {
	return systemapi.NewHandlers(a.services.System).HandleSystemRecoveryConfirm()
}

func (a *serviceHarness) handleSystemRuntimeBootstrap() http.HandlerFunc {
	return systemapi.NewHandlers(a.services.System).HandleSystemRuntimeBootstrap()
}

func (a *serviceHarness) handlePluginWebhook() http.HandlerFunc {
	return a.services.PluginWebhooks.HandleWebhook()
}

func applyConfigApplyEffects(app *serviceHarness, newCfg config.Config) configapi.ApplyEffects {
	if app == nil {
		return configapi.NewApplyEffects()
	}
	service := httpwire.NewConfigService(httpwire.ConfigDeps{
		Runtime:          app.state,
		Logs:             app.platform.Logs,
		LogRepository:    app.platform.LogRepository,
		Renderer:         app.renderStack.Renderer,
		PluginLogLimiter: app.pluginStack.PluginLogLimiter,
		OutboundLimiter:  app.eventStack.OutboundLimiter,
		Protocol:         app.services.Protocol,
		EventIngress:     app.services.EventIngress,
	})
	return configapi.NewHandlers(service).ApplyHotReloadableFields(newCfg)
}

func applyHotReloadableFields(app *serviceHarness, newCfg config.Config) bool {
	return applyConfigApplyEffects(app, newCfg).RestartRequired()
}

func newPluginWebhookRegistry() *pluginwebhook.Registry {
	return pluginwebhook.NewRegistry()
}

func newPluginLogLimiter(cfg config.Config) *localaction.PluginLogLimiter {
	return localaction.NewPluginLogLimiter(cfg)
}

func (a *serviceHarness) currentCapabilityView() localaction.CapabilityView {
	if a == nil {
		return nil
	}
	if a.capabilities != nil {
		return a.capabilities
	}
	if a.pluginStack.Plugins == nil {
		a.capabilities = &stubCapabilityView{capabilities: map[string][]stubCapability{}}
		return a.capabilities
	}
	a.capabilities = plugincapabilityview.New(plugincapabilityview.Deps{Plugins: a.pluginStack.Plugins})
	return a.capabilities
}

type stubCapability struct {
	PluginID   string
	Capability string
	ScopeJSON  string
}

type stubCapabilityView struct {
	capabilities map[string][]stubCapability
}

func stubCapabilityViewFor(pluginID string, capabilities ...string) *stubCapabilityView {
	view := &stubCapabilityView{capabilities: map[string][]stubCapability{}}
	for _, capability := range capabilities {
		view.capabilities[pluginID] = append(view.capabilities[pluginID], stubCapability{
			PluginID:   pluginID,
			Capability: capability,
		})
	}
	return view
}

func (v *stubCapabilityView) CapabilityDeclared(_ context.Context, pluginID string, capability string) bool {
	if v == nil {
		return false
	}
	for _, item := range v.capabilities[pluginID] {
		if item.Capability == capability {
			return true
		}
	}
	return false
}

func (v *stubCapabilityView) StorageRootAllowed(_ context.Context, pluginID string, root string) bool {
	for _, item := range v.capabilities[pluginID] {
		if item.Capability != "storage.file" {
			continue
		}
		for _, declared := range parseStubScopeList(item.ScopeJSON, "storage_roots") {
			if declared == root {
				return true
			}
		}
	}
	return false
}

func (v *stubCapabilityView) HTTPHosts(_ context.Context, pluginID string) []string {
	for _, item := range v.capabilities[pluginID] {
		if item.Capability == "http.request" {
			return parseStubScopeList(item.ScopeJSON, "http_hosts")
		}
	}
	return nil
}

func (v *stubCapabilityView) ThirdPartyAccountPlatforms(_ context.Context, pluginID string) []string {
	for _, item := range v.capabilities[pluginID] {
		if item.Capability == "thirdparty.account.read" {
			return parseStubScopeList(item.ScopeJSON, "third_party_account_platforms")
		}
	}
	return nil
}

func (v *stubCapabilityView) WebhookParameters(_ context.Context, pluginID string, route string) (plugins.WebhookScope, bool) {
	for _, item := range v.capabilities[pluginID] {
		if item.Capability != "event.expose_webhook" {
			continue
		}
		for _, scope := range parseStubWebhookScopes(item.ScopeJSON) {
			if scope.Route == route {
				return scope, true
			}
		}
	}
	return plugins.WebhookScope{}, false
}

func (v *stubCapabilityView) ListPluginSnapshots() []plugins.Snapshot {
	return nil
}

func parseStubScopeList(scopeJSON string, key string) []string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(scopeJSON), &payload); err != nil {
		return nil
	}
	raw, ok := payload[key].([]any)
	if !ok {
		return nil
	}
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		if value, ok := item.(string); ok && value != "" {
			values = append(values, value)
		}
	}
	return values
}

func parseStubWebhookScopes(scopeJSON string) []plugins.WebhookScope {
	var payload struct {
		Webhooks []plugins.WebhookScope `json:"webhooks"`
	}
	if err := json.Unmarshal([]byte(scopeJSON), &payload); err != nil {
		return nil
	}
	return payload.Webhooks
}

func (a *serviceHarness) dispatchPluginConfigChanged(ctx context.Context, pluginID string) {
	if a == nil {
		return
	}
	dispatch := localaction.ConfigChangedDispatcher(a.eventStack.Dispatcher)
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
