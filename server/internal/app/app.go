package app

import (
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app/eventstack"
	"github.com/RayleaBot/RayleaBot/server/internal/app/httpwire"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
	"github.com/RayleaBot/RayleaBot/server/internal/app/renderstack"
	"github.com/RayleaBot/RayleaBot/server/internal/app/servicegraph"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	runtimeregistry "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/registry"
	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/system"
)

type Options struct {
	ConfigPath            string
	SchemaPath            string
	AuthOptions           []auth.Option
	PluginRepoRoot        string
	PluginSchemaPath      string
	PluginRoots           []plugindiscovery.ScanRoot
	RenderRunner          renderbrowser.Runner
	BilibiliHTTPTransport http.RoundTripper
	BilibiliClock         func() time.Time
}

type App struct {
	state       *appRuntimeState
	process     appProcessState
	platform    appPlatform
	pluginStack appPlugins
	renderStack appRender
	eventStack  appEvents
	services    appServices

	runtimes *runtimeregistry.Registry

	httpHandlers appHTTPHandlers

	metrics                 *metrics.Registry
	metricsRuntimeGaugeStop func()
}

func New(options Options) (*App, error) {
	buildState, err := initializeAppBuild(options)
	if err != nil {
		return nil, err
	}

	schedulerTriggers := appplatform.NewTriggerProxy()
	platformState, err := appplatform.Build(appplatform.Deps{
		ConfigPath:       buildState.options.ConfigPath,
		Config:           buildState.core.Config,
		Logger:           buildState.core.Logger,
		AuthOptions:      buildState.options.AuthOptions,
		Tasks:            buildState.taskRegistry,
		TaskExecutor:     buildState.taskExecutor,
		Logs:             buildState.logStream,
		SchedulerTrigger: schedulerTriggers.Handle,
	})
	if err != nil {
		return nil, err
	}
	var (
		pluginState           pluginstack.State
		renderState           renderstack.State
		eventState            eventstack.State
		stopRuntimeStateGauge func()
	)
	cleanupPartialBuild := func() {
		partial := &App{
			platform:                platformState,
			pluginStack:             pluginState,
			renderStack:             renderState,
			eventStack:              eventState,
			metricsRuntimeGaugeStop: stopRuntimeStateGauge,
		}
		_ = partial.Close()
	}

	pluginState, err = pluginstack.Build(pluginstack.Deps{
		Config:    buildState.core.Config,
		Logger:    buildState.core.Logger,
		Discovery: buildState.discoverySpec,
		Validator: buildState.pluginValidator,
		Catalog:   buildState.pluginCatalog,
		Tasks:     buildState.taskRegistry,
		Platform:  platformState,
	})
	if err != nil {
		cleanupPartialBuild()
		return nil, err
	}

	renderState, err = renderstack.Build(renderstack.Deps{
		Config:    buildState.core.Config,
		Logger:    buildState.core.Logger,
		Discovery: buildState.discoverySpec,
		Store:     platformState.Storage,
		Catalog:   pluginState.Plugins,
		Runner:    options.RenderRunner,
	})
	if err != nil {
		cleanupPartialBuild()
		return nil, err
	}

	eventState = eventstack.Build(eventstack.Deps{
		Config: buildState.core.Config,
		Logger: buildState.core.Logger,
	})

	state := newAppRuntimeState(buildState)
	metricRegistry, stopRuntimeStateGauge := wireMetrics(platformState, eventState, renderState.Renderer, pluginState)
	serviceBuild, err := servicegraph.Build(servicegraph.BuildDeps{
		Runtime:               state,
		Platform:              platformState,
		Plugins:               pluginState,
		Events:                eventState,
		Renderer:              renderState.Renderer,
		Metrics:               metricRegistry,
		Discovery:             buildState.discoverySpec,
		PluginValidator:       buildState.pluginValidator,
		ManagementRedact:      buildState.managementRedact,
		BilibiliHTTPTransport: options.BilibiliHTTPTransport,
		BilibiliClock:         options.BilibiliClock,
	})
	if err != nil {
		cleanupPartialBuild()
		return nil, err
	}

	application := &App{
		state:                   state,
		platform:                platformState,
		pluginStack:             pluginState,
		renderStack:             renderState,
		eventStack:              eventState,
		services:                serviceBuild.Services,
		runtimes:                serviceBuild.Runtimes,
		metrics:                 metricRegistry,
		metricsRuntimeGaugeStop: stopRuntimeStateGauge,
	}
	configureAppRuntimeCallbacks(application, schedulerTriggers)
	httpState := httpwire.Build(httpwire.BuildDeps{
		Runtime:               state,
		Platform:              platformState,
		Plugins:               pluginState,
		Events:                eventState,
		Renderer:              renderState.Renderer,
		Services:              serviceBuild.Services,
		Status:                serviceBuild.Status,
		BilibiliAccountClient: serviceBuild.BilibiliAccountClient,
		BilibiliQRLogin:       serviceBuild.BilibiliQRLogin,
		Metrics:               metricRegistry,
		BilibiliHTTPTransport: options.BilibiliHTTPTransport,
		RequestShutdown:       application.requestShutdown,
	})
	application.process.router = httpState.Router
	application.process.server = httpState.Server
	application.httpHandlers = httpState.Handlers
	return application, nil
}

type readinessProvider interface {
	CurrentReadiness() health.ReadinessReport
}

var _ readinessProvider = (*systemsvc.Service)(nil)
