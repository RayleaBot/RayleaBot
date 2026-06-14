package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/eventingress"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	bilibilisource "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/source"
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
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type appServiceBuildResult struct {
	services              appServices
	runtimes              *runtimeregistry.Registry
	status                *managementevents.ServiceStatusService
	bilibiliAccountClient *bilibilisession.AccountClient
	bilibiliQRLogin       *bilibilisession.QRLoginService
}

func buildAppServices(
	buildState appBuildState,
	runtimeState *appRuntimeState,
	platform appPlatform,
	pluginStack appPlugins,
	metricRegistry *metrics.Registry,
	options Options,
) (appServiceBuildResult, error) {
	logService := logging.NewManagementService(platform.Logs, platform.LogRepository)
	grantView := buildPluginGrantView(runtimeState, pluginStack)
	governanceEvents := managementevents.NewGovernanceService()
	bilibiliEvents := managementevents.NewBilibiliSourceService()
	governanceService := buildGovernanceService(runtimeState, pluginStack, governanceEvents)
	thirdPartyService, err := thirdparty.NewService(platform.Storage, platform.Secrets)
	if err != nil {
		return appServiceBuildResult{}, err
	}
	bilibiliSession := bilibilisession.NewSessionClient(options.BilibiliHTTPTransport, options.BilibiliClock, nil)
	localActions := buildLocalActionService(runtimeState, platform, pluginStack, grantView, governanceService, thirdPartyService, bilibiliSession)
	configureLocalActionService(localActions, pluginStack)
	runtimeRegistry := buildRuntimeRegistryForApp(buildState, runtimeState, platform, localActions)
	systemService := systemsvc.New(systemsvc.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		CurrentSummary:   func() config.Summary { return runtimeState.Summary },
		CurrentRepoRoot:  func() string { return runtimeState.repoRoot },
		CurrentStartedAt: func() time.Time { return runtimeState.startedAt },
		Logger:           runtimeState.Logger,
		Auth:             platform.Auth,
		Adapter:          pluginStack.Adapter,
		Plugins:          pluginStack.Plugins,
		Runtimes:         runtimeRegistry,
		Renderer:         pluginStack.renderer,
		Storage:          platform.Storage,
		PluginRepository: pluginStack.pluginRepository,
		TaskExecutor:     platform.taskExecutor,
		LogRepository:    platform.LogRepository,
	})
	serviceStatusService := managementevents.NewServiceStatusService(systemService)
	systemService.SetStatusPublisher(serviceStatusService)
	lifecycle := pluginservice.NewController(pluginservice.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		RepoRoot:         runtimeState.repoRoot,
		Logger:           runtimeState.Logger,
		Plugins:          pluginStack.Plugins,
		DesiredStateRepo: pluginStack.pluginRepository,
		Grants:           grantView,
		Runtimes:         runtimeRegistry,
		Dispatcher:       pluginStack.Dispatcher,
		Scheduler:        platform.Scheduler,
		PluginConfig:     pluginStack.pluginConfig,
		Adapter:          pluginStack.Adapter,
		Webhooks:         pluginStack.webhooks,
		Tasks:            platform.Tasks,
		OnRecoveryChange: systemService.ReconcileRecoverySummaryBestEffort,
		RefreshManifest:  buildPluginLifecycleRefreshManifest(buildState, runtimeState, pluginStack),
		SyncRenderTemplates: func(ctx context.Context) error {
			return renderplugintemplates.SyncCatalogRenderTemplates(ctx, pluginStack.renderer, pluginStack.Plugins)
		},
	})
	menuService := buildBuiltinMenuService(runtimeState, pluginStack)
	eventIngress := eventingress.New(eventingress.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		Logger:           runtimeState.Logger,
		Plugins:          pluginStack.Plugins,
		ReplyTargets:     pluginStack.replyTargets,
		OutboundSender:   pluginStack.outboundSender,
		OutboundLimiter:  pluginStack.outboundLimiter,
		Renderer:         pluginStack.renderer,
		Menu:             menuService,
		Bridge:           pluginStack.Bridge,
		Lifecycle:        lifecycle,
		MetadataEnricher: pluginStack.Adapter,
		WhitelistRepo:    pluginStack.whitelistRepo,
		WhitelistState:   pluginStack.whitelistState,
		BlacklistRepo:    pluginStack.blacklistRepo,
	})
	protocolService := protocolapi.NewService(runtimeState, pluginStack.Adapter)
	pluginWebhooks := buildPluginWebhookGateway(runtimeState, platform, pluginStack, lifecycle, grantView)
	pluginWebhooks.SetReplayMetrics(metrics.NewWebhookReplayObserver(metricRegistry))
	localActions.SetWebhookGateway(pluginWebhooks)
	bilibiliSource, err := buildBilibiliSourceService(platform, pluginStack, thirdPartyService, bilibiliSession, bilibiliEvents, options)
	if err != nil {
		return appServiceBuildResult{}, err
	}

	return appServiceBuildResult{
		services: appServices{
			localActions:     localActions,
			pluginLifecycle:  lifecycle,
			eventIngress:     eventIngress,
			protocol:         protocolService,
			pluginWebhooks:   pluginWebhooks,
			governance:       governanceService,
			governanceEvents: governanceEvents,
			logs:             logService,
			system:           systemService,
			thirdParty:       thirdPartyService,
			bilibiliSource:   bilibiliSource,
			bilibiliEvents:   bilibiliEvents,
		},
		runtimes:              runtimeRegistry,
		status:                serviceStatusService,
		bilibiliAccountClient: bilibilisession.NewAccountClient(options.BilibiliHTTPTransport, options.BilibiliClock, nil),
		bilibiliQRLogin:       bilibilisession.NewQRLoginService(options.BilibiliHTTPTransport, options.BilibiliClock),
	}, nil
}

func buildPluginGrantView(runtimeState *appRuntimeState, pluginStack appPlugins) *plugingrants.View {
	grantView := plugingrants.NewView(plugingrants.ViewDeps{
		Plugins:               pluginStack.Plugins,
		GrantRepository:       pluginStack.grantRepository,
		AutoGrantCapabilities: currentPluginAutoGrantCapabilities(runtimeState),
	})
	pluginStack.Dispatcher.SetCapabilityChecker(grantView.CapabilityGranted)
	return grantView
}

func buildGovernanceService(runtimeState *appRuntimeState, pluginStack appPlugins, events *managementevents.GovernanceService) *governance.Service {
	return governance.NewService(governance.Deps{
		CurrentConfig:  func() config.Config { return runtimeState.Config },
		Plugins:        pluginStack.Plugins,
		BlacklistRepo:  pluginStack.blacklistRepo,
		WhitelistRepo:  pluginStack.whitelistRepo,
		WhitelistState: pluginStack.whitelistState,
		NotifyChanged:  events.PublishChanged,
	})
}

func buildLocalActionService(
	runtimeState *appRuntimeState,
	platform appPlatform,
	pluginStack appPlugins,
	grantView *plugingrants.View,
	governanceService *governance.Service,
	thirdPartyService *thirdparty.Service,
	bilibiliSession *bilibilisession.SessionClient,
) *localaction.Service {
	return localaction.New(localaction.Deps{
		CurrentConfig:    func() config.Config { return runtimeState.Config },
		Logger:           runtimeState.Logger,
		RedactText:       runtimeState.redactString,
		Grants:           grantView,
		PluginConfig:     pluginStack.pluginConfig,
		PluginFiles:      pluginStack.pluginFiles,
		PluginKV:         pluginStack.pluginKV,
		Secrets:          localActionSecretReader(platform.Secrets),
		Scheduler:        localActionScheduler(platform.Scheduler),
		Dispatcher:       localActionConfigChangedDispatcher(pluginStack.Dispatcher),
		Renderer:         localActionRenderer(pluginStack.renderer),
		Adapter:          pluginStack.Adapter,
		PluginLogLimiter: pluginStack.pluginLogLimiter,
		Governance:       governanceService,
		ThirdParty:       thirdPartyService,
		BilibiliSession:  bilibiliSession,
	})
}

func configureLocalActionService(localActions *localaction.Service, pluginStack appPlugins) {
	localActions.SetRefreshPluginCommands(func(ctx context.Context, pluginID string, settings map[string]any) {
		lifecyclecommands.RefreshPluginCommands(pluginStack.Plugins, pluginStack.Dispatcher, pluginID, settings)
	})
}

func localActionScheduler(engine *scheduler.Engine) localaction.SchedulerCreateFunc {
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

func localActionConfigChangedDispatcher(dispatcher *dispatch.Dispatcher) localaction.ConfigChangeDispatcher {
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

func localActionSecretReader(store secrets.Store) localaction.SecretReader {
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

func localActionRenderer(service *renderservice.Service) localaction.Renderer {
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
	buildState appBuildState,
	runtimeState *appRuntimeState,
	pluginStack appPlugins,
) func(context.Context, string) (plugins.Snapshot, error) {
	return func(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
		return pluginmanifestrefresh.RefreshPluginManifest(ctx, pluginStack.Plugins, pluginStack.pluginConfig, pluginID, func() ([]plugins.Snapshot, error) {
			snapshots, _, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
				Validator: buildState.pluginValidator,
				Roots:     buildState.discoverySpec.Roots,
				RepoRoot:  buildState.discoverySpec.RepoRoot,
				Logger:    runtimeState.Logger,
			})
			if err != nil {
				return nil, err
			}
			if packageLoader, ok := any(pluginStack.pluginRepository).(plugins.PackageMetadataLoader); ok {
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
	runtimeState *appRuntimeState,
	platform appPlatform,
	pluginStack appPlugins,
	lifecycle *pluginservice.Controller,
	grantView *plugingrants.View,
) *pluginwebhook.Service {
	return pluginwebhook.New(pluginwebhook.Deps{
		CurrentConfig: func() config.Config { return runtimeState.Config },
		Logger:        runtimeState.Logger,
		Registry:      pluginStack.webhooks,
		Secrets:       platform.Secrets,
		Plugins:       pluginStack.Plugins,
		Dispatcher:    pluginStack.Dispatcher,
		Runtime:       lifecycle,
		Grants:        grantView,
	})
}

func buildBilibiliSourceService(
	platform appPlatform,
	pluginStack appPlugins,
	thirdPartyService *thirdparty.Service,
	bilibiliSession *bilibilisession.SessionClient,
	bilibiliEvents *managementevents.BilibiliSourceService,
	options Options,
) (*bilibilisource.Source, error) {
	return bilibilisource.NewSource(bilibilisource.Deps{
		Store:         bilibilisource.Store{Read: platform.Storage.Read, Write: platform.Storage.Write},
		Accounts:      thirdPartyService,
		PluginConfig:  pluginStack.pluginConfig,
		Dispatcher:    bilibiliEventDispatcher{dispatcher: pluginStack.Dispatcher},
		NotifyStatus:  bilibiliEvents.Publish,
		HTTPTransport: options.BilibiliHTTPTransport,
		Session:       bilibiliSession,
		Now:           options.BilibiliClock,
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
