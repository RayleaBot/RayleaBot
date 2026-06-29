package pluginmodule

import (
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/extensions/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
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
