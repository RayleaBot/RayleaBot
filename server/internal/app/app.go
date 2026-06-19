package app

import (
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app/httpwire"
	appplatform "github.com/RayleaBot/RayleaBot/server/internal/app/platform"
	"github.com/RayleaBot/RayleaBot/server/internal/app/pluginstack"
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

	pluginState, err := pluginstack.Build(pluginstack.Deps{
		Config:       buildState.core.Config,
		Logger:       buildState.core.Logger,
		Discovery:    buildState.discoverySpec,
		Validator:    buildState.pluginValidator,
		Catalog:      buildState.pluginCatalog,
		Tasks:        buildState.taskRegistry,
		Platform:     platformState,
		RenderRunner: options.RenderRunner,
	})
	if err != nil {
		return nil, err
	}

	state := newAppRuntimeState(buildState)
	metricRegistry, stopRuntimeStateGauge := pluginstack.WireMetrics(platformState, pluginState)
	serviceBuild, err := servicegraph.Build(servicegraph.BuildDeps{
		Runtime:               state,
		Platform:              platformState,
		Plugins:               pluginState,
		Metrics:               metricRegistry,
		Discovery:             buildState.discoverySpec,
		PluginValidator:       buildState.pluginValidator,
		ManagementRedact:      buildState.managementRedact,
		BilibiliHTTPTransport: options.BilibiliHTTPTransport,
		BilibiliClock:         options.BilibiliClock,
	})
	if err != nil {
		return nil, err
	}

	application := &App{
		state:                   state,
		platform:                platformState,
		pluginStack:             pluginState,
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

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

type readinessProvider interface {
	CurrentReadiness() health.ReadinessReport
}

var _ readinessProvider = (*systemsvc.Service)(nil)
var _ http.Handler = (http.Handler)(nil)
