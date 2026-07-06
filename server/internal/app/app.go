package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

type Options struct {
	ConfigPath            string
	SchemaPath            string
	AuthOptions           []auth.Option
	PluginRepoRoot        string
	PluginSchemaPath      string
	PluginRoots           []plugincatalog.ScanRoot
	RenderRunner          renderservice.Runner
	BilibiliHTTPTransport http.RoundTripper
	BilibiliClock         func() time.Time
}

type App struct {
	state       *appRuntimeState
	process     appProcessState
	platform    PlatformState
	pluginStack PluginStackState
	renderStack appRenderState
	eventStack  EventState
	services    Services

	runtimes *pluginruntime.Registry

	httpHandlers httpHandlers

	metrics                 *MetricsRegistry
	metricsRuntimeGaugeStop func()
}

func New(options Options) (*App, error) {
	return NewWithContext(context.Background(), options)
}

func NewWithContext(ctx context.Context, options Options) (*App, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	buildState, err := initializeAppBuild(options)
	if err != nil {
		return nil, err
	}

	schedulerTriggers := newSchedulerTriggerProxy()
	platformState, err := buildPlatform(platformDeps{
		Context:          ctx,
		ConfigPath:       buildState.options.ConfigPath,
		Config:           buildState.core.CurrentConfig(),
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
		pluginState           PluginStackState
		renderState           appRenderState
		eventState            EventState
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
	resolvedConfig, err := configruntime.ResolveConfigSecretRefs(ctx, platformState.Secrets, buildState.core.CurrentConfig())
	if err != nil {
		cleanupPartialBuild()
		return nil, fmt.Errorf("resolve config secrets: %w", err)
	}
	buildState.core.SetConfig(resolvedConfig)
	buildState.core.AddRedactionValues(configruntime.ConfigSecretValues(resolvedConfig)...)

	pluginState, err = buildPluginStack(pluginStackDeps{
		Context:   ctx,
		Config:    resolvedConfig,
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

	renderState, err = buildRender(renderDeps{
		Context:   ctx,
		Config:    resolvedConfig,
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

	eventState = buildEvents(eventDeps{
		Config: resolvedConfig,
		Logger: buildState.core.Logger,
	})

	state := buildState.core
	metricRegistry, stopRuntimeStateGauge := wireMetrics(platformState, eventState, renderState.Renderer, pluginState)
	serviceBuild, err := buildServices(serviceBuildDeps{
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
	httpState := buildHTTP(httpBuildDeps{
		Runtime:         state,
		Platform:        platformState,
		Plugins:         pluginState,
		Events:          eventState,
		Renderer:        renderState.Renderer,
		ServiceBuild:    serviceBuild,
		Metrics:         metricRegistry,
		RequestShutdown: application.requestShutdown,
	})
	application.process.router = httpState.Router
	application.process.server = httpState.Server
	application.httpHandlers = httpState.Handlers
	return application, nil
}

func wireMetrics(platform PlatformState, events EventState, renderer *renderservice.Service, plugins PluginStackState) (*MetricsRegistry, func()) {
	registry := NewMetricsRegistry()
	events.Bridge.SetMetricsObserver(NewBridgeObserver(registry))
	events.Dispatcher.SetMetricsObserver(NewDispatchObserver(registry))
	events.Adapter.SetMetricsObserver(NewAdapterObserver(registry))
	platform.TaskExecutor.SetMetricsObserver(NewTaskObserver(registry))
	renderer.SetMetricsObserver(NewRenderObserver(registry))
	return registry, StartPluginStateGaugeRefresh(registry, plugins.Plugins)
}
