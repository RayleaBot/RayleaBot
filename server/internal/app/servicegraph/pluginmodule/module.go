package pluginmodule

import (
	"context"
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/app/actionwire"
	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/console"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugincapabilityview "github.com/RayleaBot/RayleaBot/server/internal/plugins/capabilityview"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	lifecyclecommands "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle/commands"
	pluginmanifestrefresh "github.com/RayleaBot/RayleaBot/server/internal/plugins/manifestrefresh"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
	runtimeregistry "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/registry"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderplugintemplates "github.com/RayleaBot/RayleaBot/server/internal/render/plugintemplates"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
)

type LocalActionService = localaction.Service
type LifecycleController = pluginservice.Controller
type WebhookService = pluginwebhook.Service
type RuntimeRegistry = runtimeregistry.Registry
type MenuService = menuext.Service

type Module struct {
	Runtime  Runtime
	Services Services
}

type RuntimeState interface {
	CurrentConfig() config.Config
	RuntimeLogger() *slog.Logger
	RedactString(string) string
	RepoRoot() string
}

type RuntimeDeps struct {
	Runtime          RuntimeState
	Platform         appplatform.State
	Plugins          pluginstack.State
	Events           eventstack.State
	Renderer         *renderservice.Service
	Governance       *governance.Service
	ManagementRedact func(string) string
	HTTPCredentials  localaction.HTTPCredentialInjector
}

type Runtime struct {
	LocalActions   *LocalActionService
	Runtimes       *RuntimeRegistry
	CapabilityView *plugincapabilityview.View
}

func BuildRuntime(deps RuntimeDeps) Runtime {
	capabilityView := buildPluginCapabilityView(deps.Plugins, deps.Events)
	localActions := buildLocalActionService(deps.Runtime, deps.Platform, deps.Plugins, deps.Events, deps.Renderer, capabilityView, deps.Governance, deps.HTTPCredentials)
	configureLocalActionService(localActions, deps.Plugins, deps.Events)
	runtimeRegistry := buildRuntimeRegistry(runtimeRegistryDeps{
		Logger:                     deps.Runtime.RuntimeLogger(),
		Console:                    deps.Platform.Console,
		RedactText:                 deps.ManagementRedact,
		StderrRateLimitBytesPerSec: deps.Runtime.CurrentConfig().Runtime.StderrRateLimitBytesPerSec,
		ExecuteLocalAction:         localActions.Execute,
	})
	return Runtime{
		LocalActions:   localActions,
		Runtimes:       runtimeRegistry,
		CapabilityView: capabilityView,
	}
}

type ServiceDeps struct {
	Runtime         RuntimeState
	Platform        appplatform.State
	Plugins         pluginstack.State
	Events          eventstack.State
	Renderer        *renderservice.Service
	System          *systemsvc.Service
	Discovery       runtimepaths.PluginDiscoverySpec
	PluginValidator *schema.Validator
	PluginRuntime   Runtime
	Metrics         *metrics.Registry
}

type Services struct {
	PluginLifecycle *LifecycleController
	PluginWebhooks  *WebhookService
	Menu            *MenuService
}

func BuildServices(deps ServiceDeps) Services {
	lifecycle := buildPluginLifecycle(deps)
	menu := buildBuiltinMenuService(deps.Runtime, deps.Plugins, deps.Events, deps.Renderer)
	pluginWebhooks := buildPluginWebhookGateway(deps.Runtime, deps.Platform, deps.Plugins, deps.Events, lifecycle, deps.PluginRuntime.CapabilityView)
	pluginWebhooks.SetReplayMetrics(metrics.NewWebhookReplayObserver(deps.Metrics))
	deps.PluginRuntime.LocalActions.SetWebhookGateway(pluginWebhooks)
	return Services{
		PluginLifecycle: lifecycle,
		PluginWebhooks:  pluginWebhooks,
		Menu:            menu,
	}
}

func buildPluginCapabilityView(pluginStack pluginstack.State, eventStack eventstack.State) *plugincapabilityview.View {
	capabilityView := plugincapabilityview.New(plugincapabilityview.Deps{
		Plugins: pluginStack.Plugins,
	})
	if eventStack.Dispatcher != nil {
		eventStack.Dispatcher.SetCapabilityChecker(capabilityView.CapabilityDeclared)
	}
	return capabilityView
}

func buildLocalActionService(
	runtimeState RuntimeState,
	platform appplatform.State,
	pluginStack pluginstack.State,
	eventStack eventstack.State,
	renderer *renderservice.Service,
	capabilityView *plugincapabilityview.View,
	governanceService *governance.Service,
	httpCredentials localaction.HTTPCredentialInjector,
) *localaction.Service {
	return localaction.New(localaction.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		Logger:           runtimeState.RuntimeLogger(),
		RedactText:       runtimeState.RedactString,
		Capabilities:     capabilityView,
		PluginConfig:     pluginStack.PluginConfig,
		PluginFiles:      pluginStack.PluginFiles,
		PluginKV:         pluginStack.PluginKV,
		Secrets:          actionwire.SecretReader(platform.Secrets),
		Scheduler:        actionwire.Scheduler(platform.Scheduler),
		Dispatcher:       actionwire.ConfigChangedDispatcher(eventStack.Dispatcher),
		Renderer:         actionwire.Renderer(renderer),
		Adapter:          eventStack.Adapter,
		PluginLogLimiter: pluginStack.PluginLogLimiter,
		Governance:       governanceService,
		HTTPCredentials:  httpCredentials,
	})
}

func configureLocalActionService(localActions *localaction.Service, pluginStack pluginstack.State, eventStack eventstack.State) {
	localActions.SetRefreshPluginCommands(func(ctx context.Context, pluginID string, settings map[string]any) {
		lifecyclecommands.RefreshPluginCommands(pluginStack.Plugins, eventStack.Dispatcher, pluginID, settings)
	})
}

type runtimeRegistryDeps struct {
	Logger                     *slog.Logger
	Console                    *console.Stream
	RedactText                 func(string) string
	StderrRateLimitBytesPerSec int
	ExecuteLocalAction         runtimemanager.LocalActionExecutor
}

func buildRuntimeRegistry(deps runtimeRegistryDeps) *runtimeregistry.Registry {
	return runtimeregistry.New(deps.Logger, runtimemanager.Options{
		Console:                    deps.Console,
		RedactText:                 deps.RedactText,
		StderrRateLimitBytesPerSec: deps.StderrRateLimitBytesPerSec,
		ExecuteLocalAction:         deps.ExecuteLocalAction,
	})
}

func buildPluginLifecycle(deps ServiceDeps) *pluginservice.Controller {
	return pluginservice.NewController(pluginservice.Deps{
		CurrentConfig:    deps.Runtime.CurrentConfig,
		RepoRoot:         deps.Runtime.RepoRoot(),
		Logger:           deps.Runtime.RuntimeLogger(),
		Plugins:          deps.Plugins.Plugins,
		DesiredStateRepo: deps.Plugins.PluginRepository,
		Runtimes:         deps.PluginRuntime.Runtimes,
		Dispatcher:       deps.Events.Dispatcher,
		Scheduler:        deps.Platform.Scheduler,
		PluginConfig:     deps.Plugins.PluginConfig,
		Adapter:          deps.Events.Adapter,
		Webhooks:         deps.Plugins.Webhooks,
		Tasks:            deps.Platform.Tasks,
		OnRecoveryChange: deps.System.ReconcileRecoverySummaryBestEffort,
		RefreshManifest:  buildPluginLifecycleRefreshManifest(deps),
		SyncRenderTemplates: func(ctx context.Context) error {
			return renderplugintemplates.SyncCatalogRenderTemplates(ctx, deps.Renderer, deps.Plugins.Plugins)
		},
	})
}

func buildPluginLifecycleRefreshManifest(deps ServiceDeps) func(context.Context, string) (plugins.Snapshot, error) {
	return func(ctx context.Context, pluginID string) (plugins.Snapshot, error) {
		return pluginmanifestrefresh.RefreshPluginManifest(ctx, deps.Plugins.Plugins, deps.Plugins.PluginConfig, pluginID, func() ([]plugins.Snapshot, error) {
			snapshots, _, err := plugindiscovery.Discover(plugindiscovery.DiscoverOptions{
				Validator: deps.PluginValidator,
				Roots:     deps.Discovery.Roots,
				RepoRoot:  deps.Discovery.RepoRoot,
				Logger:    deps.Runtime.RuntimeLogger(),
			})
			if err != nil {
				return nil, err
			}
			if packageLoader, ok := any(deps.Plugins.PluginRepository).(plugins.PackageMetadataLoader); ok {
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

func buildBuiltinMenuService(runtimeState RuntimeState, pluginStack pluginstack.State, eventStack eventstack.State, renderer *renderservice.Service) *menuext.Service {
	return menuext.New(menuext.Deps{
		CurrentConfig: runtimeState.CurrentConfig,
		Plugins:       pluginStack.Plugins,
		Renderer:      renderer,
		Sender:        eventStack.OutboundSender,
		WaitOutbound: func(ctx context.Context, request outbound.MessageLimitRequest) error {
			if eventStack.OutboundLimiter == nil {
				return nil
			}
			return eventStack.OutboundLimiter.Wait(ctx, request)
		},
		Logger: runtimeState.RuntimeLogger(),
	})
}

func buildPluginWebhookGateway(
	runtimeState RuntimeState,
	platform appplatform.State,
	pluginStack pluginstack.State,
	eventStack eventstack.State,
	lifecycle *pluginservice.Controller,
	capabilityView pluginwebhook.CapabilityView,
) *pluginwebhook.Service {
	return pluginwebhook.New(pluginwebhook.Deps{
		CurrentConfig: runtimeState.CurrentConfig,
		Logger:        runtimeState.RuntimeLogger(),
		Registry:      pluginStack.Webhooks,
		Secrets:       platform.Secrets,
		Plugins:       pluginStack.Plugins,
		Dispatcher:    eventStack.Dispatcher,
		Runtime:       lifecycle,
		Capabilities:  capabilityView,
	})
}
