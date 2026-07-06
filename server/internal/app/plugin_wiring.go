package app

import (
	"context"
	"errors"

	menuext "github.com/RayleaBot/RayleaBot/server/internal/builtinmenu"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
)

type pluginRuntimeDeps struct {
	Runtime          runtimeStateView
	Platform         PlatformState
	Plugins          PluginStackState
	Events           EventState
	Renderer         *renderservice.Service
	Governance       *governance.Service
	ManagementRedact func(string) string
	ThirdParty       localaction.ThirdPartyAccountReader
}

type pluginRuntime struct {
	LocalActions   *localaction.Service
	Runtimes       *pluginruntime.Registry
	CapabilityView *plugins.CapabilityView
}

func buildPluginRuntime(deps pluginRuntimeDeps) pluginRuntime {
	capabilityView := buildPluginCapabilityView(deps.Plugins, deps.Events)
	localActions := buildLocalActionService(deps.Runtime, deps.Platform, deps.Plugins, deps.Events, deps.Renderer, capabilityView, deps.Governance, deps.ThirdParty)
	runtimeRegistry := pluginruntime.NewManaged(
		deps.Runtime.RuntimeLogger(),
		deps.Platform.Console,
		deps.ManagementRedact,
		deps.Runtime.CurrentConfig().Runtime.StderrRateLimitBytesPerSec,
		localActions.Execute,
	)
	return pluginRuntime{
		LocalActions:   localActions,
		Runtimes:       runtimeRegistry,
		CapabilityView: capabilityView,
	}
}

func buildPluginCapabilityView(pluginStack PluginStackState, eventStack EventState) *plugins.CapabilityView {
	capabilityView := plugins.NewCapabilityView(plugins.CapabilityViewDeps{
		Plugins: pluginStack.Plugins,
	})
	if eventStack.Dispatcher != nil {
		eventStack.Dispatcher.SetCapabilityChecker(capabilityView.CapabilityDeclared)
	}
	return capabilityView
}

func buildLocalActionService(
	runtimeState runtimeStateView,
	platform PlatformState,
	pluginStack PluginStackState,
	eventStack EventState,
	renderer *renderservice.Service,
	capabilityView *plugins.CapabilityView,
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
	})
}

type pluginServiceDeps struct {
	Runtime       runtimeStateView
	Platform      PlatformState
	Plugins       PluginStackState
	Events        EventState
	Renderer      *renderservice.Service
	System        *systemsvc.Service
	PluginRuntime pluginRuntime
	Metrics       *MetricsRegistry
}

type pluginServices struct {
	PluginLifecycle *pluginservice.Controller
	PluginWebhooks  *pluginwebhook.Service
	Menu            *menuext.Service
}

func buildPluginServices(deps pluginServiceDeps) (pluginServices, error) {
	lifecycle := buildPluginLifecycle(deps)
	menu, err := buildBuiltinMenuService(deps.Runtime, deps.Plugins, deps.Events, deps.Renderer)
	if err != nil {
		return pluginServices{}, err
	}
	pluginWebhooks := buildPluginWebhookGateway(deps.Runtime, deps.Platform, deps.Plugins, deps.Events, lifecycle, deps.PluginRuntime.CapabilityView)
	pluginWebhooks.SetReplayMetrics(NewWebhookReplayObserver(deps.Metrics))
	deps.PluginRuntime.LocalActions.SetWebhookGateway(pluginWebhooks)
	return pluginServices{
		PluginLifecycle: lifecycle,
		PluginWebhooks:  pluginWebhooks,
		Menu:            menu,
	}, nil
}

func buildPluginLifecycle(deps pluginServiceDeps) *pluginservice.Controller {
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

func buildBuiltinMenuService(runtimeState runtimeStateView, pluginStack PluginStackState, eventStack EventState, renderer *renderservice.Service) (*menuext.Service, error) {
	if eventStack.OutboundSender == nil {
		return nil, errors.New("builtin menu requires an outbound sender")
	}
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
	}), nil
}

func buildPluginWebhookGateway(
	runtimeState runtimeStateView,
	platform PlatformState,
	pluginStack PluginStackState,
	eventStack EventState,
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

func pluginRenderTemplateSync(deps pluginServiceDeps) func(context.Context) error {
	return func(ctx context.Context) error {
		if deps.Renderer == nil || deps.Plugins.Plugins == nil {
			return nil
		}
		return deps.Renderer.SyncPluginTemplateDeclarations(ctx, pluginRenderTemplateDeclarations(deps.Plugins.Plugins.List()))
	}
}
