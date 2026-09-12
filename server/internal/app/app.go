package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/bridge"
	configruntime "github.com/RayleaBot/RayleaBot/server/internal/config/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/filelock"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/runtimepaths"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

type Options struct {
	ConfigPath              string
	SchemaPath              string
	SetupToken              string
	LauncherControlToken    string
	DevelopmentArtifactRoot string
	AuthOptions             []auth.Option
	PluginRepoRoot          string
	PluginSchemaPath        string
	PluginRoots             []plugincatalog.ScanRoot
	RenderRunner            render.Runner
	// LogRepository overrides the SQLite-backed management log repository.
	// Test-only seam; nil means the default repository is built.
	LogRepository logging.Repository
	// BridgeDispatch overrides the dispatcher the event bridge talks to.
	// Test-only seam; nil means the real dispatcher is used.
	BridgeDispatch bridge.Dispatch
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
	configLifecycleLock     *filelock.Lock
}

func New(options Options) (*App, error) {
	return NewWithContext(context.Background(), options)
}

// normalizeOptions fills the generated tokens and resolves the development
// artifact root so the rest of assembly sees validated inputs.
func normalizeOptions(options Options) (Options, error) {
	var err error
	if options.SetupToken, err = ensureOpaqueToken(options.SetupToken, "setup token"); err != nil {
		return options, err
	}
	if options.LauncherControlToken, err = ensureOpaqueToken(options.LauncherControlToken, "launcher control token"); err != nil {
		return options, err
	}
	if options.DevelopmentArtifactRoot != "" {
		if !filepath.IsAbs(options.DevelopmentArtifactRoot) {
			return options, errors.New("development artifact root must be absolute")
		}
		canonical, err := filepath.EvalSymlinks(options.DevelopmentArtifactRoot)
		if err != nil {
			return options, fmt.Errorf("resolve development artifact root: %w", err)
		}
		options.DevelopmentArtifactRoot = canonical
	}
	return options, nil
}

func ensureOpaqueToken(token, label string) (string, error) {
	if token == "" {
		generated, err := auth.GenerateOpaqueToken(32)
		if err != nil {
			return "", fmt.Errorf("generate %s: %w", label, err)
		}
		token = generated
	}
	if err := auth.ValidateOpaqueToken(token); err != nil {
		return "", fmt.Errorf("validate %s: %w", label, err)
	}
	return token, nil
}

func NewWithContext(ctx context.Context, options Options) (*App, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	options, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}
	lockPath, err := runtimepaths.ResolveConfigLifecycleLockPath(options.ConfigPath)
	if err != nil {
		return nil, err
	}
	configLifecycleLock, err := filelock.Acquire(lockPath)
	if err != nil {
		if errors.Is(err, filelock.ErrLocked) {
			return nil, fmt.Errorf("config is already in use by a running service: %s", options.ConfigPath)
		}
		return nil, fmt.Errorf("acquire config lifecycle lock: %w", err)
	}
	lockTransferred := false
	defer func() {
		if !lockTransferred {
			_ = configLifecycleLock.Close()
		}
	}()

	buildState, err := initializeAppBuild(options)
	if err != nil {
		return nil, err
	}

	var schedulerLifecycle interface {
		HandleSchedulerTrigger(context.Context, scheduler.Job)
	}
	schedulerTrigger := func(ctx context.Context, job scheduler.Job) {
		if schedulerLifecycle == nil {
			return
		}
		schedulerLifecycle.HandleSchedulerTrigger(ctx, job)
	}
	var (
		platformState         PlatformState
		pluginState           PluginStackState
		renderState           appRenderState
		eventState            EventState
		serviceBuild          serviceBuildResult
		stopRuntimeStateGauge func()
	)
	// cleanupPartialBuild releases whatever has been assembled so far by
	// closing it as an App; stages that have not run leave their zero value.
	cleanupPartialBuild := func(cause error) error {
		partial := &App{
			platform:                platformState,
			pluginStack:             pluginState,
			renderStack:             renderState,
			eventStack:              eventState,
			services:                serviceBuild.Services,
			runtimes:                serviceBuild.Runtimes,
			metricsRuntimeGaugeStop: stopRuntimeStateGauge,
		}
		return errors.Join(cause, partial.Close())
	}
	platformState, err = buildPlatform(platformDeps{
		Context:          ctx,
		ConfigPath:       buildState.options.ConfigPath,
		Config:           buildState.core.CurrentConfig(),
		Logger:           buildState.core.Logger,
		AuthOptions:      buildState.options.AuthOptions,
		Tasks:            buildState.taskRegistry,
		TaskExecutor:     buildState.taskExecutor,
		Logs:             buildState.logStream,
		LogRepository:    options.LogRepository,
		SchedulerTrigger: schedulerTrigger,
	})
	if err != nil {
		platformState = PlatformState{
			TaskExecutor: buildState.taskExecutor,
			Tasks:        buildState.taskRegistry,
			Logs:         buildState.logStream,
		}
		return nil, cleanupPartialBuild(err)
	}
	resolvedConfig, err := configruntime.ResolveConfigSecretRefs(ctx, platformState.Secrets, buildState.core.CurrentConfig())
	if err != nil {
		return nil, cleanupPartialBuild(fmt.Errorf("resolve config secrets: %w", err))
	}
	buildState.core.SetConfig(resolvedConfig)
	buildState.core.AddRedactionValues(configruntime.ConfigSecretValues(resolvedConfig)...)

	pluginDeps := pluginStackDeps{
		Context:   ctx,
		Config:    resolvedConfig,
		Logger:    buildState.core.Logger,
		Discovery: buildState.discoverySpec,
		Validator: buildState.pluginValidator,
		Catalog:   buildState.pluginCatalog,
		Tasks:     buildState.taskRegistry,
		Platform:  platformState,
	}
	pluginState, err = buildPluginStack(pluginDeps)
	if err != nil {
		return nil, cleanupPartialBuild(err)
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
		return nil, cleanupPartialBuild(err)
	}

	eventState = buildEvents(eventDeps{
		Config:         resolvedConfig,
		CurrentConfig:  buildState.core.CurrentConfig,
		Logger:         buildState.core.Logger,
		BridgeDispatch: options.BridgeDispatch,
	})

	state := buildState.core
	metricRegistry, stopRuntimeStateGauge := wireMetrics(platformState, eventState, renderState.Renderer, pluginState)
	serviceBuild, err = buildServices(serviceBuildDeps{
		Runtime:          state,
		Platform:         platformState,
		Plugins:          pluginState,
		Events:           eventState,
		Renderer:         renderState.Renderer,
		Metrics:          metricRegistry,
		Discovery:        buildState.discoverySpec,
		PluginValidator:  buildState.pluginValidator,
		ManagementRedact: buildState.managementRedact,
	})
	if err != nil {
		return nil, cleanupPartialBuild(err)
	}
	if serviceBuild.Services.PluginLifecycle == nil {
		return nil, cleanupPartialBuild(fmt.Errorf("plugin lifecycle service is required"))
	}
	schedulerLifecycle = serviceBuild.Services.PluginLifecycle
	if err := buildPluginMutationServices(pluginDeps, &pluginState, serviceBuild.Services, renderState.Renderer); err != nil {
		return nil, cleanupPartialBuild(err)
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
		configLifecycleLock:     configLifecycleLock,
	}
	configureAppRuntimeCallbacks(application)
	httpState, err := buildHTTP(httpBuildDeps{
		Runtime:                 state,
		Platform:                platformState,
		Plugins:                 pluginState,
		Events:                  eventState,
		Renderer:                renderState.Renderer,
		ServiceBuild:            serviceBuild,
		Metrics:                 metricRegistry,
		RequestShutdown:         application.requestShutdown,
		SetupToken:              options.SetupToken,
		LauncherControlToken:    options.LauncherControlToken,
		DevelopmentArtifactRoot: options.DevelopmentArtifactRoot,
	})
	if err != nil {
		return nil, errors.Join(err, application.Close())
	}
	application.process.router = httpState.Router
	application.process.server = httpState.Server
	application.httpHandlers = httpState.Handlers
	lockTransferred = true
	return application, nil
}

func wireMetrics(platform PlatformState, events EventState, renderer *render.Service, plugins PluginStackState) (*MetricsRegistry, func()) {
	registry := NewMetricsRegistry()
	events.Bridge.SetMetricsObserver(NewBridgeObserver(registry))
	events.Dispatcher.SetMetricsObserver(NewDispatchObserver(registry))
	for _, shell := range events.OneBotShells {
		shell.SetMetricsObserver(NewAdapterObserver(registry))
	}
	platform.TaskExecutor.SetMetricsObserver(NewTaskObserver(registry))
	renderer.SetMetricsObserver(NewRenderObserver(registry))
	return registry, StartPluginStateGaugeRefresh(registry, plugins.Plugins)
}
