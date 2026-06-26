package pluginmodule

import (
	"context"
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	defaultactionmodules "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/defaultmodules"
	plugincapabilityview "github.com/RayleaBot/RayleaBot/server/internal/plugins/capabilityview"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	runtimeregistry "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/registry"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
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
	ThirdParty       localaction.ThirdPartyAccountReader
}

type Runtime struct {
	LocalActions   *LocalActionService
	Runtimes       *RuntimeRegistry
	CapabilityView *plugincapabilityview.View
}

func BuildRuntime(deps RuntimeDeps) Runtime {
	capabilityView := buildPluginCapabilityView(deps.Plugins, deps.Events)
	localActions := buildLocalActionService(deps.Runtime, deps.Platform, deps.Plugins, deps.Events, deps.Renderer, capabilityView, deps.Governance, deps.ThirdParty)
	runtimeRegistry := runtimeregistry.NewManaged(
		deps.Runtime.RuntimeLogger(),
		deps.Platform.Console,
		deps.ManagementRedact,
		deps.Runtime.CurrentConfig().Runtime.StderrRateLimitBytesPerSec,
		localActions.Execute,
	)
	return Runtime{
		LocalActions:   localActions,
		Runtimes:       runtimeRegistry,
		CapabilityView: capabilityView,
	}
}

type ServiceDeps struct {
	Runtime       RuntimeState
	Platform      appplatform.State
	Plugins       pluginstack.State
	Events        eventstack.State
	Renderer      *renderservice.Service
	System        *systemsvc.Service
	PluginRuntime Runtime
	Metrics       *metrics.Registry
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
	thirdParty localaction.ThirdPartyAccountReader,
) *localaction.Service {
	return localaction.New(localaction.Deps{
		CurrentConfig:    runtimeState.CurrentConfig,
		Logger:           runtimeState.RuntimeLogger(),
		RedactText:       runtimeState.RedactString,
		Capabilities:     capabilityView,
		PluginConfig:     pluginStack.PluginConfig,
		PluginFiles:      pluginStack.PluginFiles,
		PluginKV:         pluginStack.PluginKV,
		Secrets:          localaction.SecretReaderFromStore(platform.Secrets),
		ThirdParty:       thirdParty,
		Scheduler:        localaction.Scheduler(platform.Scheduler),
		Dispatcher:       localaction.ConfigChangedDispatcher(eventStack.Dispatcher),
		Renderer:         localaction.RendererFromService(renderer),
		Adapter:          eventStack.Adapter,
		PluginLogLimiter: pluginStack.PluginLogLimiter,
		Governance:       governanceService,
		RefreshCommands:  localaction.RefreshCommands(pluginStack.Plugins, eventStack.Dispatcher),
		Registrars:       defaultactionmodules.Registrars(),
	})
}

func buildPluginLifecycle(deps ServiceDeps) *pluginservice.Controller {
	return pluginservice.NewController(pluginservice.Deps{
		CurrentConfig:       deps.Runtime.CurrentConfig,
		RepoRoot:            deps.Runtime.RepoRoot(),
		Logger:              deps.Runtime.RuntimeLogger(),
		Plugins:             deps.Plugins.Plugins,
		DesiredStateRepo:    deps.Plugins.PluginRepository,
		Runtimes:            deps.PluginRuntime.Runtimes,
		Dispatcher:          deps.Events.Dispatcher,
		Scheduler:           deps.Platform.Scheduler,
		PluginConfig:        deps.Plugins.PluginConfig,
		Adapter:             deps.Events.Adapter,
		Webhooks:            deps.Plugins.Webhooks,
		Tasks:               deps.Platform.Tasks,
		OnRecoveryChange:    deps.System.ReconcileRecoverySummaryBestEffort,
		RefreshManifest:     deps.Plugins.RefreshManifest,
		SyncRenderTemplates: pluginRenderTemplateSync(deps),
	})
}

func pluginRenderTemplateSync(deps ServiceDeps) func(context.Context) error {
	return func(ctx context.Context) error {
		if deps.Renderer == nil || deps.Plugins.Plugins == nil {
			return nil
		}
		return deps.Renderer.SyncPluginTemplateDeclarations(ctx, pluginRenderTemplateDeclarations(deps.Plugins.Plugins.List()))
	}
}

func pluginRenderTemplateDeclarations(snapshots []plugins.Snapshot) []renderservice.PluginTemplateDeclaration {
	var declarations []renderservice.PluginTemplateDeclaration
	for _, snapshot := range snapshots {
		for _, declared := range snapshot.RenderTemplates {
			declarations = append(declarations, renderservice.PluginTemplateDeclaration{
				PluginID:          snapshot.PluginID,
				Path:              declared.Path,
				PackageRootPath:   snapshot.PackageRootPath,
				Valid:             snapshot.Valid,
				RegistrationState: snapshot.RegistrationState,
			})
		}
	}
	return declarations
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
