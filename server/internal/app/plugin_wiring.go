package app

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/governance"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/bot/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/outbound"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/system"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	pluginservice "github.com/RayleaBot/RayleaBot/server/internal/plugins/lifecycle"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/settings"
	pluginwebhook "github.com/RayleaBot/RayleaBot/server/internal/plugins/webhook"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

type pluginRuntimeDeps struct {
	Runtime          runtimeStateView
	Platform         PlatformState
	Plugins          PluginStackState
	Events           EventState
	Renderer         *render.Service
	Governance       *governance.Service
	ManagementRedact func(string) string
	Browser          localaction.BrowserSessionManager
}

type pluginRuntime struct {
	LocalActions   *localaction.Service
	Settings       *settings.Service
	Runtimes       *pluginruntime.Registry
	PermissionView *plugins.PermissionView
}

func buildPluginRuntime(deps pluginRuntimeDeps) (pluginRuntime, error) {
	if deps.Plugins.Plugins == nil || deps.Plugins.PluginConfig == nil || deps.Platform.Secrets == nil || deps.Events.Dispatcher == nil {
		return pluginRuntime{}, errors.New("plugin settings dependencies are required")
	}
	settingsService, err := settings.New(settings.Deps{
		Plugins:         deps.Plugins.Plugins,
		Config:          deps.Plugins.PluginConfig,
		Secrets:         deps.Platform.Secrets,
		RefreshCommands: localaction.RefreshCommands(deps.Plugins.Plugins, deps.Events.Dispatcher),
		Notify:          localaction.NotifyConfigChanged(deps.Events.Dispatcher),
	})
	if err != nil {
		return pluginRuntime{}, err
	}
	permissionView := buildPluginPermissionView(deps.Plugins, deps.Events)
	localActions := buildLocalActionService(deps.Runtime, deps.Platform, deps.Plugins, deps.Events, deps.Renderer, permissionView, deps.Governance, deps.Browser, settingsService)
	runtimeRegistry := pluginruntime.NewManaged(
		deps.Runtime.RuntimeLogger(),
		deps.Platform.Console,
		deps.ManagementRedact,
		deps.Runtime.CurrentConfig().Runtime.StderrRateLimitBytesPerSec,
		localActions.Execute,
	)
	return pluginRuntime{
		LocalActions:   localActions,
		Settings:       settingsService,
		Runtimes:       runtimeRegistry,
		PermissionView: permissionView,
	}, nil
}

func buildPluginPermissionView(pluginStack PluginStackState, eventStack EventState) *plugins.PermissionView {
	permissionView := plugins.NewPermissionView(plugins.PermissionViewDeps{
		Plugins: pluginStack.Plugins,
	})
	if eventStack.Dispatcher != nil {
		eventStack.Dispatcher.SetPermissionChecker(permissionView.PermissionDeclared)
	}
	return permissionView
}

func buildLocalActionService(
	runtimeState runtimeStateView,
	platform PlatformState,
	pluginStack PluginStackState,
	eventStack EventState,
	renderer *render.Service,
	permissionView *plugins.PermissionView,
	governanceService *governance.Service,
	browserManager localaction.BrowserSessionManager,
	settingsService *settings.Service,
) *localaction.Service {
	return localaction.New(localaction.Deps{
		CurrentConfig:        runtimeState.CurrentConfig,
		Logger:               runtimeState.RuntimeLogger(),
		RedactText:           runtimeState.RedactString,
		Permissions:          permissionView,
		Plugins:              pluginStack.Plugins,
		Settings:             settingsService,
		PluginFiles:          pluginStack.PluginFiles,
		PluginKV:             pluginStack.PluginKV,
		Browser:              browserManager,
		Scheduler:            localaction.Scheduler(platform.Scheduler),
		MessageSender:        localaction.OutboundMessageSender(eventStack.Dispatcher),
		Renderer:             localaction.RendererFromService(renderer),
		ResolveOneBotAdapter: eventStack.ResolveOneBotAdapter,
		PluginLogLimiter:     pluginStack.PluginLogLimiter,
		Governance:           governanceService,
	})
}

type pluginServiceDeps struct {
	Runtime       runtimeStateView
	Platform      PlatformState
	Plugins       PluginStackState
	Events        EventState
	Renderer      *render.Service
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
	lifecycle, err := buildPluginLifecycle(deps)
	if err != nil {
		return pluginServices{}, err
	}
	menu, err := buildBuiltinMenuService(deps.Runtime, deps.Plugins, deps.Events, deps.Renderer)
	if err != nil {
		return pluginServices{}, err
	}
	pluginWebhooks, err := buildPluginWebhookGateway(deps.Runtime, deps.Platform, deps.Plugins, deps.Events, lifecycle)
	if err != nil {
		return pluginServices{}, err
	}
	pluginWebhooks.SetReplayMetrics(NewWebhookReplayObserver(deps.Metrics))
	pluginWebhooks.SyncManifestRegistrations()
	return pluginServices{
		PluginLifecycle: lifecycle,
		PluginWebhooks:  pluginWebhooks,
		Menu:            menu,
	}, nil
}

func buildPluginLifecycle(deps pluginServiceDeps) (*pluginservice.Controller, error) {
	return pluginservice.NewController(pluginservice.Deps{
		CurrentConfig:       deps.Runtime.CurrentConfig,
		RepoRoot:            deps.Runtime.RepoRoot(),
		Logger:              deps.Runtime.RuntimeLogger(),
		Plugins:             deps.Plugins.Plugins,
		DesiredStateRepo:    deps.Plugins.PluginRepository,
		Runtimes:            deps.PluginRuntime.Runtimes,
		Dispatcher:          deps.Events.Dispatcher,
		Scheduler:           deps.Platform.Scheduler,
		Settings:            deps.PluginRuntime.Settings,
		Identities:          deps.Events.BotIdentity,
		Webhooks:            deps.Plugins.Webhooks,
		Tasks:               deps.Platform.Tasks,
		OnRecoveryChange:    deps.System.ReconcileRecoverySummaryBestEffort,
		RefreshManifest:     deps.Plugins.RefreshManifest,
		SyncRenderTemplates: pluginRenderTemplateSync(deps),
		Operations:          deps.Plugins.Operations,
	})
}

func buildBuiltinMenuService(runtimeState runtimeStateView, pluginStack PluginStackState, eventStack EventState, renderer *render.Service) (*menuext.Service, error) {
	if eventStack.OutboundSender == nil {
		return nil, errors.New("builtin menu requires an outbound sender")
	}
	var menuRenderer menuext.Renderer
	if renderer != nil {
		menuRenderer = func(ctx context.Context, request menuext.RenderRequest) (string, error) {
			result, err := renderer.Render(ctx, render.Request{
				Template: "help.menu", Data: request.Data,
				Plugin: &render.PluginContext{Name: request.PluginName, Version: request.PluginVersion},
			})
			return result.ImagePath, err
		}
	}
	return menuext.New(menuext.Deps{
		CurrentConfig: runtimeState.CurrentConfig,
		Plugins:       pluginStack.Plugins,
		Renderer:      menuRenderer,
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
) (*pluginwebhook.Service, error) {
	return pluginwebhook.New(pluginwebhook.Deps{
		Logger:     runtimeState.RuntimeLogger(),
		Registry:   pluginStack.Webhooks,
		Secrets:    platform.Secrets,
		Plugins:    pluginStack.Plugins,
		Dispatcher: eventStack.Dispatcher,
		Runtime:    lifecycle,
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
