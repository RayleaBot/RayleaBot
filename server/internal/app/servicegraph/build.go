package servicegraph

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/eventingress"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	bilibilicredential "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/credential"
	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/source"
	bilibilisubscriptions "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/subscriptions"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/management/protocolapi"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	plugingrants "github.com/RayleaBot/RayleaBot/server/internal/plugins/grants"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	lifecyclecommands "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle/commands"
	pluginmanifestrefresh "github.com/RayleaBot/RayleaBot/server/internal/plugins/manifestrefresh"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	runtimeregistry "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/registry"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderplugintemplates "github.com/RayleaBot/RayleaBot/server/internal/render/plugintemplates"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	rendertemplates "github.com/RayleaBot/RayleaBot/server/internal/render/templates"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type RuntimeState interface {
	CurrentConfig() config.Config
	CurrentSummary() config.Summary
	RepoRoot() string
	StartedAt() time.Time
	RuntimeLogger() *slog.Logger
	RedactString(string) string
}

type BuildDeps struct {
	Runtime               RuntimeState
	Platform              appplatform.State
	Plugins               pluginstack.State
	Metrics               *metrics.Registry
	Discovery             runtimepaths.PluginDiscoverySpec
	PluginValidator       *schema.Validator
	ManagementRedact      func(string) string
	BilibiliHTTPTransport http.RoundTripper
	BilibiliClock         func() time.Time
}

type Services struct {
	LocalActions     *localaction.Service
	PluginLifecycle  *pluginservice.Controller
	EventIngress     *eventingress.Service
	Protocol         *protocolapi.Service
	PluginWebhooks   *pluginwebhook.Service
	Governance       *governance.Service
	GovernanceEvents *managementevents.GovernanceService
	Logs             *logging.ManagementService
	System           *systemsvc.Service
	ThirdParty       *thirdparty.Service
	BilibiliSource   *bilibilisource.Source
	BilibiliEvents   *managementevents.BilibiliSourceService
}

type BuildResult struct {
	Services              Services
	Runtimes              *runtimeregistry.Registry
	Status                *managementevents.ServiceStatusService
	BilibiliAccountClient *bilibilisession.AccountClient
	BilibiliQRLogin       *bilibilisession.QRLoginService
}

func Build(deps BuildDeps) (BuildResult, error) {
	runtimeState := deps.Runtime
	platform := deps.Platform
	pluginStack := deps.Plugins
	logService := logging.NewManagementService(platform.Logs, platform.LogRepository)
	grantView := buildPluginGrantView(runtimeState, pluginStack)
	governanceEvents := managementevents.NewGovernanceService()
	bilibiliEvents := managementevents.NewBilibiliSourceService()
	governanceService := buildGovernanceService(runtimeState, pluginStack, governanceEvents)
	thirdPartyService, err := thirdparty.NewService(platform.Storage, platform.Secrets)
	if err != nil {
		return BuildResult{}, err
	}
	bilibiliSession := bilibilisession.NewSessionClient(deps.BilibiliHTTPTransport, deps.BilibiliClock, nil)
	localActions := buildLocalActionService(runtimeState, platform, pluginStack, grantView, governanceService, thirdPartyService, bilibiliSession)
	configureLocalActionService(localActions, pluginStack)
	runtimeRegistry := pluginstack.BuildRuntimeRegistry(pluginstack.RuntimeRegistryDeps{
		Logger:                     runtimeState.RuntimeLogger(),
		Console:                    platform.Console,
		RedactText:                 deps.ManagementRedact,
		StderrRateLimitBytesPerSec: runtimeState.CurrentConfig().Runtime.StderrRateLimitBytesPerSec,
		ExecuteLocalAction:         localActions.Execute,
	})
	systemService := systemsvc.New(systemsvc.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		CurrentSummary:   runtimeState.CurrentSummary,
		CurrentRepoRoot:  runtimeState.RepoRoot,
		CurrentStartedAt: runtimeState.StartedAt,
		Logger:           runtimeState.RuntimeLogger(),
		Auth:             platform.Auth,
		Adapter:          pluginStack.Adapter,
		Plugins:          pluginStack.Plugins,
		Runtimes:         runtimeRegistry,
		Renderer:         pluginStack.Renderer,
		Storage:          platform.Storage,
		PluginRepository: pluginStack.PluginRepository,
		TaskExecutor:     platform.TaskExecutor,
		LogRepository:    platform.LogRepository,
	})
	serviceStatusService := managementevents.NewServiceStatusService(systemService)
	systemService.SetStatusPublisher(serviceStatusService)
	lifecycle := pluginservice.NewController(pluginservice.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		RepoRoot:         runtimeState.RepoRoot(),
		Logger:           runtimeState.RuntimeLogger(),
		Plugins:          pluginStack.Plugins,
		DesiredStateRepo: pluginStack.PluginRepository,
		Grants:           grantView,
		Runtimes:         runtimeRegistry,
		Dispatcher:       pluginStack.Dispatcher,
		Scheduler:        platform.Scheduler,
		PluginConfig:     pluginStack.PluginConfig,
		Adapter:          pluginStack.Adapter,
		Webhooks:         pluginStack.Webhooks,
		Tasks:            platform.Tasks,
		OnRecoveryChange: systemService.ReconcileRecoverySummaryBestEffort,
		RefreshManifest:  buildPluginLifecycleRefreshManifest(deps, pluginStack),
		SyncRenderTemplates: func(ctx context.Context) error {
			return renderplugintemplates.SyncCatalogRenderTemplates(ctx, pluginStack.Renderer, pluginStack.Plugins)
		},
	})
	menuService := pluginstack.BuildBuiltinMenuService(pluginstack.MenuDeps{
		CurrentConfig: runtimeState.CurrentConfig,
		Logger:        runtimeState.RuntimeLogger(),
		Plugins:       pluginStack,
	})
	eventIngress := eventingress.New(eventingress.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		Logger:           runtimeState.RuntimeLogger(),
		Plugins:          pluginStack.Plugins,
		ReplyTargets:     pluginStack.ReplyTargets,
		OutboundSender:   pluginStack.OutboundSender,
		OutboundLimiter:  pluginStack.OutboundLimiter,
		Renderer:         pluginStack.Renderer,
		Menu:             menuService,
		Bridge:           pluginStack.Bridge,
		Lifecycle:        lifecycle,
		MetadataEnricher: pluginStack.Adapter,
		WhitelistRepo:    pluginStack.WhitelistRepo,
		WhitelistState:   pluginStack.WhitelistState,
		BlacklistRepo:    pluginStack.BlacklistRepo,
	})
	protocolService := protocolapi.NewService(runtimeState, pluginStack.Adapter)
	pluginWebhooks := buildPluginWebhookGateway(runtimeState, platform, pluginStack, lifecycle, grantView)
	pluginWebhooks.SetReplayMetrics(metrics.NewWebhookReplayObserver(deps.Metrics))
	localActions.SetWebhookGateway(pluginWebhooks)
	bilibiliSource, err := buildBilibiliSourceService(platform, pluginStack, thirdPartyService, bilibiliSession, bilibiliEvents, deps)
	if err != nil {
		return BuildResult{}, err
	}

	return BuildResult{
		Services: Services{
			LocalActions:     localActions,
			PluginLifecycle:  lifecycle,
			EventIngress:     eventIngress,
			Protocol:         protocolService,
			PluginWebhooks:   pluginWebhooks,
			Governance:       governanceService,
			GovernanceEvents: governanceEvents,
			Logs:             logService,
			System:           systemService,
			ThirdParty:       thirdPartyService,
			BilibiliSource:   bilibiliSource,
			BilibiliEvents:   bilibiliEvents,
		},
		Runtimes:              runtimeRegistry,
		Status:                serviceStatusService,
		BilibiliAccountClient: bilibilisession.NewAccountClient(deps.BilibiliHTTPTransport, deps.BilibiliClock, nil),
		BilibiliQRLogin:       bilibilisession.NewQRLoginService(deps.BilibiliHTTPTransport, deps.BilibiliClock),
	}, nil
}

func buildPluginGrantView(runtimeState RuntimeState, pluginStack pluginstack.State) *plugingrants.View {
	grantView := plugingrants.NewView(plugingrants.ViewDeps{
		Plugins:               pluginStack.Plugins,
		GrantRepository:       pluginStack.GrantRepository,
		AutoGrantCapabilities: autoGrantCapabilities(runtimeState),
	})
	pluginStack.Dispatcher.SetCapabilityChecker(grantView.CapabilityGranted)
	return grantView
}

func buildGovernanceService(runtimeState RuntimeState, pluginStack pluginstack.State, events *managementevents.GovernanceService) *governance.Service {
	return governance.NewService(governance.Deps{
		CurrentConfig:  runtimeState.CurrentConfig,
		Plugins:        pluginStack.Plugins,
		BlacklistRepo:  pluginStack.BlacklistRepo,
		WhitelistRepo:  pluginStack.WhitelistRepo,
		WhitelistState: pluginStack.WhitelistState,
		NotifyChanged:  events.PublishChanged,
	})
}

func buildLocalActionService(
	runtimeState RuntimeState,
	platform appplatform.State,
	pluginStack pluginstack.State,
	grantView *plugingrants.View,
	governanceService *governance.Service,
	thirdPartyService *thirdparty.Service,
	bilibiliSession *bilibilisession.SessionClient,
) *localaction.Service {
	return localaction.New(localaction.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		Logger:           runtimeState.RuntimeLogger(),
		RedactText:       runtimeState.RedactString,
		Grants:           grantView,
		PluginConfig:     pluginStack.PluginConfig,
		PluginFiles:      pluginStack.PluginFiles,
		PluginKV:         pluginStack.PluginKV,
		Secrets:          LocalActionSecretReader(platform.Secrets),
		Scheduler:        LocalActionScheduler(platform.Scheduler),
		Dispatcher:       LocalActionConfigChangedDispatcher(pluginStack.Dispatcher),
		Renderer:         LocalActionRenderer(pluginStack.Renderer),
		Adapter:          pluginStack.Adapter,
		PluginLogLimiter: pluginStack.PluginLogLimiter,
		Governance:       governanceService,
		HTTPCredentials:  bilibilicredential.NewInjector(thirdPartyService, bilibiliSession),
	})
}

func configureLocalActionService(localActions *localaction.Service, pluginStack pluginstack.State) {
	localActions.SetRefreshPluginCommands(func(ctx context.Context, pluginID string, settings map[string]any) {
		lifecyclecommands.RefreshPluginCommands(pluginStack.Plugins, pluginStack.Dispatcher, pluginID, settings)
	})
}

func LocalActionScheduler(engine *scheduler.Engine) localaction.SchedulerCreateFunc {
	if engine == nil {
		return nil
	}
	return func(ctx context.Context, pluginID, taskID, logLabel, cron string, payload []byte) (localaction.ScheduledTask, error) {
		job, err := engine.UpsertTaskWithLabel(ctx, pluginID, taskID, logLabel, cron, payload)
		if err != nil {
			return localaction.ScheduledTask{}, err
		}
		return localaction.ScheduledTask{
			JobID:   job.JobID,
			NextRun: job.NextRun,
		}, nil
	}
}

func LocalActionConfigChangedDispatcher(dispatcher *dispatch.Dispatcher) localaction.ConfigChangeDispatcher {
	if dispatcher == nil {
		return nil
	}
	return func(ctx context.Context, pluginID string) localaction.ConfigChangeDispatchResult {
		if !dispatcher.HasDeliverablePlugin(pluginID) {
			return localaction.ConfigChangeDispatchResult{Delivered: true}
		}
		result := dispatcher.DispatchToPlugin(ctx, pluginID, runtimeprotocol.Event{
			EventID:        fmt.Sprintf("config-changed-%s-%d", pluginID, time.Now().UnixNano()),
			SourceProtocol: "platform",
			SourceAdapter:  "config.internal",
			EventType:      "config.changed",
			Timestamp:      time.Now().Unix(),
			Target: &runtimeprotocol.EventTarget{
				Type: "plugin",
				ID:   pluginID,
				Name: pluginID,
			},
		})
		return localaction.ConfigChangeDispatchResult{
			Delivered: result.Outcome == dispatch.OutcomeDelivered,
			Outcome:   string(result.Outcome),
			ErrorCode: result.ErrorCode,
		}
	}
}

type localActionSecrets struct {
	store secrets.Store
}

func LocalActionSecretReader(store secrets.Store) localaction.SecretReader {
	if store == nil {
		return nil
	}
	return localActionSecrets{store: store}
}

func (s localActionSecrets) ReadPluginSecret(ctx context.Context, storageKey string) (string, bool, error) {
	value, err := s.store.Get(ctx, storageKey)
	if err != nil {
		if errors.Is(err, secrets.ErrNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	plaintext, err := secrets.OpenString(ctx, s.store, value)
	if err != nil {
		return "", false, err
	}
	return plaintext, true, nil
}

type localActionRenderService struct {
	service *renderservice.Service
}

func LocalActionRenderer(service *renderservice.Service) localaction.Renderer {
	if service == nil {
		return nil
	}
	return localActionRenderService{service: service}
}

func (r localActionRenderService) ResolvePluginTemplate(ctx context.Context, pluginID, templatePath string) (string, error) {
	templateID, err := r.service.ResolvePluginTemplate(ctx, pluginID, templatePath)
	if err == nil {
		return templateID, nil
	}
	var renderErr *rendertemplates.Error
	if errors.As(err, &renderErr) {
		return "", &localaction.RenderTemplateError{
			Code:    renderErr.Code,
			Message: renderErr.Message,
			Err:     err,
		}
	}
	return "", err
}

func (r localActionRenderService) RenderImage(ctx context.Context, req localaction.RenderImageRequest) (localaction.RenderImageResult, error) {
	result, err := r.service.Render(ctx, renderservice.Request{
		Template: req.Template,
		Theme:    req.Theme,
		Output:   req.Output,
		Data:     req.Data,
		Plugin: &renderservice.PluginContext{
			Name:    req.Plugin.Name,
			Version: req.Plugin.Version,
		},
	})
	if err != nil {
		return localaction.RenderImageResult{}, err
	}
	return localaction.RenderImageResult{
		ArtifactID: result.ArtifactID,
		ImagePath:  result.ImagePath,
		MIME:       result.MIME,
		CacheKey:   result.CacheKey,
	}, nil
}

func (r localActionRenderService) TemplateAcceptsRenderIdentity(ctx context.Context, templateID string) bool {
	_, source, err := r.service.GetTemplateSource(ctx, templateID)
	if err != nil {
		return false
	}
	properties, ok := source.InputSchemaJSON["properties"].(map[string]any)
	if !ok {
		return false
	}
	_, hasUser := properties["user"]
	_, hasPermission := properties["permission"]
	return hasUser && hasPermission
}

func buildPluginLifecycleRefreshManifest(
	deps BuildDeps,
	pluginStack pluginstack.State,
) func(context.Context, string) (plugins.Snapshot, error) {
	return func(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
		return pluginmanifestrefresh.RefreshPluginManifest(ctx, pluginStack.Plugins, pluginStack.PluginConfig, pluginID, func() ([]plugins.Snapshot, error) {
			snapshots, _, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
				Validator: deps.PluginValidator,
				Roots:     deps.Discovery.Roots,
				RepoRoot:  deps.Discovery.RepoRoot,
				Logger:    deps.Runtime.RuntimeLogger(),
			})
			if err != nil {
				return nil, err
			}
			if packageLoader, ok := any(pluginStack.PluginRepository).(plugins.PackageMetadataLoader); ok {
				packageMetadata, err := packageLoader.LoadAllPackageMetadata(ctx)
				if err != nil {
					return nil, err
				}
				snapshots = plugins.ApplyPackageMetadata(snapshots, packageMetadata)
			}
			return snapshots, nil
		})
	}
}

func buildPluginWebhookGateway(
	runtimeState RuntimeState,
	platform appplatform.State,
	pluginStack pluginstack.State,
	lifecycle *pluginservice.Controller,
	grantView *plugingrants.View,
) *pluginwebhook.Service {
	return pluginwebhook.New(pluginwebhook.Deps{
		CurrentConfig: runtimeState.CurrentConfig,
		Logger:        runtimeState.RuntimeLogger(),
		Registry:      pluginStack.Webhooks,
		Secrets:       platform.Secrets,
		Plugins:       pluginStack.Plugins,
		Dispatcher:    pluginStack.Dispatcher,
		Runtime:       lifecycle,
		Grants:        grantView,
	})
}

func buildBilibiliSourceService(
	platform appplatform.State,
	pluginStack pluginstack.State,
	thirdPartyService *thirdparty.Service,
	bilibiliSession *bilibilisession.SessionClient,
	bilibiliEvents *managementevents.BilibiliSourceService,
	deps BuildDeps,
) (*bilibilisource.Source, error) {
	return bilibilisource.NewSource(bilibilisource.Deps{
		Store:         bilibilisource.Store{Read: platform.Storage.Read, Write: platform.Storage.Write},
		Accounts:      thirdPartyService,
		Subjects:      bilibilisubscriptions.NewPluginConfigProvider(pluginStack.PluginConfig),
		Dispatcher:    bilibiliEventDispatcher{dispatcher: pluginStack.Dispatcher},
		NotifyStatus:  bilibiliEvents.Publish,
		HTTPTransport: deps.BilibiliHTTPTransport,
		Session:       bilibiliSession,
		Now:           deps.BilibiliClock,
	})
}

type bilibiliEventDispatcher struct {
	dispatcher *dispatch.Dispatcher
}

func (d bilibiliEventDispatcher) Dispatch(ctx context.Context, event runtimeprotocol.Event, commandName string) {
	if d.dispatcher == nil {
		return
	}
	d.dispatcher.Dispatch(ctx, event, commandName)
}

func autoGrantCapabilities(runtimeState RuntimeState) func() []string {
	return func() []string {
		if runtimeState == nil {
			return nil
		}
		return append([]string(nil), runtimeState.CurrentConfig().Permission.AutoGrantCapabilities...)
	}
}
