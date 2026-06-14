package apphost

import (
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/metrics"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"
	runtimeregistry "github.com/RayleaBot/RayleaBot/server/internal/runtime/registry"
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

	schedulerTriggers := newSchedulerTriggerProxy()
	platformState, err := buildAppPlatform(buildState, schedulerTriggers.Handle)
	if err != nil {
		return nil, err
	}

	pluginState, err := buildAppPlugins(buildState, platformState, options.RenderRunner)
	if err != nil {
		return nil, err
	}

	state := newAppRuntimeState(buildState)
	metricRegistry, stopRuntimeStateGauge := wireAppMetrics(platformState, pluginState)
	serviceBuild, err := buildAppServices(buildState, state, platformState, pluginState, metricRegistry, options)
	if err != nil {
		return nil, err
	}

	application := &App{
		state:                   state,
		platform:                platformState,
		pluginStack:             pluginState,
		services:                serviceBuild.services,
		runtimes:                serviceBuild.runtimes,
		metrics:                 metricRegistry,
		metricsRuntimeGaugeStop: stopRuntimeStateGauge,
	}
	configureAppRuntimeCallbacks(application, schedulerTriggers)
	configureAppHTTP(application, serviceBuild, options)
	return application, nil
}
