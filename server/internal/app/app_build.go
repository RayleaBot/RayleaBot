package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/auth"
	"rayleabot/server/internal/bridge"
	"rayleabot/server/internal/config"
	"rayleabot/server/internal/console"
	"rayleabot/server/internal/deps"
	"rayleabot/server/internal/dispatch"
	"rayleabot/server/internal/health"
	"rayleabot/server/internal/httpapi"
	"rayleabot/server/internal/logging"
	"rayleabot/server/internal/permission"
	"rayleabot/server/internal/pluginconfig"
	"rayleabot/server/internal/pluginfile"
	"rayleabot/server/internal/pluginkv"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/render"
	"rayleabot/server/internal/runtime"
	"rayleabot/server/internal/scheduler"
	"rayleabot/server/internal/schema"
	"rayleabot/server/internal/secrets"
	"rayleabot/server/internal/storage"
	"rayleabot/server/internal/tasks"
)

var resolveManagedRenderBrowserPath = func(ctx context.Context, repoRoot string) (string, error) {
	return deps.NewManager(repoRoot).ResolveEntrypoint(ctx, "chromium", "browser")
}

type appBuildState struct {
	core             appCore
	options          Options
	logStream        *logging.Stream
	taskRegistry     *tasks.Registry
	taskExecutor     *tasks.Executor
	discoverySpec    pluginDiscoverySpec
	pluginValidator  *schema.Validator
	pluginCatalog    *plugins.Catalog
	managementRedact func(string) string
}

func initializeAppBuild(options Options) (appBuildState, error) {
	cfg, summary, err := config.Load(options.ConfigPath, options.SchemaPath)
	if err != nil {
		return appBuildState{}, err
	}

	managementRedactor := buildManagementRedactor(cfg)
	logger, logStream, logLevel, err := logging.NewWithStreamAndController(cfg.Logging.Level, managementRedactor.Redact)
	if err != nil {
		return appBuildState{}, err
	}

	taskRegistry := tasks.NewRegistry()
	taskExecutor := tasks.NewExecutor(taskRegistry, 5*time.Minute)
	discoverySpec, err := resolvePluginDiscovery(options)
	if err != nil {
		return appBuildState{}, err
	}
	pluginValidator, err := schema.Compile(discoverySpec.pluginSchemaPath)
	if err != nil {
		return appBuildState{}, fmt.Errorf("compile plugin manifest schema %s: %w", discoverySpec.pluginSchemaPath, err)
	}
	snapshots, _, err := plugins.Discover(plugins.DiscoverOptions{
		Validator: pluginValidator,
		Roots:     discoverySpec.roots,
		RepoRoot:  discoverySpec.repoRoot,
		Logger:    logger,
	})
	if err != nil {
		return appBuildState{}, err
	}

	return appBuildState{
		core: appCore{
			Config:     cfg,
			Summary:    summary,
			Logger:     logger,
			LogLevel:   logLevel,
			repoRoot:   discoverySpec.repoRoot,
			redactText: managementRedactor.Redact,
			startedAt:  time.Now().UTC(),
		},
		options:          options,
		logStream:        logStream,
		taskRegistry:     taskRegistry,
		taskExecutor:     taskExecutor,
		discoverySpec:    discoverySpec,
		pluginValidator:  pluginValidator,
		pluginCatalog:    plugins.NewCatalog(snapshots),
		managementRedact: managementRedactor.Redact,
	}, nil
}

func buildAppPlatform(state appBuildState, schedulerTrigger func(context.Context, scheduler.Job)) (appPlatform, error) {
	databasePath, err := resolveDatabasePath(state.options.ConfigPath, state.core.Config.Database.Path)
	if err != nil {
		return appPlatform{}, err
	}
	if err := migrateLegacyDataRoot(state.core.Logger, state.options.ConfigPath, state.core.Config.Database.Path); err != nil {
		return appPlatform{}, err
	}

	storageStore, err := storage.Open(databasePath)
	if err != nil {
		return appPlatform{}, fmt.Errorf("open sqlite store: %w", err)
	}
	authRepository, err := auth.NewSQLiteRepository(storageStore)
	if err != nil {
		_ = storageStore.Close()
		return appPlatform{}, fmt.Errorf("create auth repository: %w", err)
	}
	secretStore, err := secrets.NewSQLiteStore(storageStore)
	if err != nil {
		_ = storageStore.Close()
		return appPlatform{}, fmt.Errorf("create secret store: %w", err)
	}
	sessionSigningKey, signingKeyCreated, err := ensureSessionSigningKey(context.Background(), secretStore)
	if err != nil {
		_ = storageStore.Close()
		return appPlatform{}, fmt.Errorf("prepare session signing key: %w", err)
	}
	if signingKeyCreated {
		persistedSessions, err := authRepository.LoadSessions(context.Background())
		if err != nil {
			_ = storageStore.Close()
			return appPlatform{}, fmt.Errorf("load persisted sessions for signing key rotation: %w", err)
		}
		if len(persistedSessions) > 0 {
			sessionIDs := make([]string, 0, len(persistedSessions))
			for _, session := range persistedSessions {
				if session.SessionID != "" {
					sessionIDs = append(sessionIDs, session.SessionID)
				}
			}
			if len(sessionIDs) > 0 {
				if err := authRepository.DeleteSessions(context.Background(), sessionIDs); err != nil {
					_ = storageStore.Close()
					return appPlatform{}, fmt.Errorf("invalidate persisted sessions after signing key rotation: %w", err)
				}
			}
		}
	}
	authOptions := append([]auth.Option{
		auth.WithRepository(authRepository),
		auth.WithSigningKey(sessionSigningKey),
	}, state.options.AuthOptions...)
	authManager, err := auth.NewManager(auth.Config{
		SessionTTLDays: state.core.Config.Auth.SessionTTLDays,
		SlidingRenewal: state.core.Config.Auth.SlidingRenewal,
		MaxSessions:    state.core.Config.Auth.MaxSessions,
	}, authOptions...)
	if err != nil {
		_ = storageStore.Close()
		return appPlatform{}, fmt.Errorf("create auth manager: %w", err)
	}

	taskRepository, err := tasks.NewSQLiteRepository(storageStore)
	if err != nil {
		_ = storageStore.Close()
		return appPlatform{}, fmt.Errorf("create task repository: %w", err)
	}
	state.taskRegistry.SetRepository(taskRepository)
	if err := state.taskRegistry.Hydrate(context.Background()); err != nil {
		_ = storageStore.Close()
		return appPlatform{}, fmt.Errorf("hydrate task registry: %w", err)
	}
	logRepository, err := logging.NewSQLiteRepository(storageStore)
	if err != nil {
		_ = storageStore.Close()
		return appPlatform{}, fmt.Errorf("create logging repository: %w", err)
	}
	state.logStream.SetRepository(logRepository, state.core.Config.Logging.RetentionDays)
	if state.core.Config.Logging.RetentionDays > 0 {
		if err := logRepository.PruneOlderThan(context.Background(), time.Now().AddDate(0, 0, -state.core.Config.Logging.RetentionDays)); err != nil {
			_ = storageStore.Close()
			return appPlatform{}, fmt.Errorf("prune persisted management logs: %w", err)
		}
	}
	schedulerRepo, err := scheduler.NewSQLiteRepository(storageStore)
	if err != nil {
		_ = storageStore.Close()
		return appPlatform{}, fmt.Errorf("create scheduler repository: %w", err)
	}
	schedulerEngine, err := scheduler.New(scheduler.Options{
		Repository: schedulerRepo,
		Logger:     state.core.Logger,
		Trigger:    schedulerTrigger,
		Timezone:   state.core.Config.Runtime.SchedulerTimezone,
	})
	if err != nil {
		_ = storageStore.Close()
		return appPlatform{}, fmt.Errorf("create scheduler engine: %w", err)
	}
	if err := schedulerEngine.Hydrate(context.Background()); err != nil {
		_ = storageStore.Close()
		return appPlatform{}, fmt.Errorf("hydrate scheduler: %w", err)
	}

	return appPlatform{
		Auth:           authManager,
		Storage:        storageStore,
		Secrets:        secretStore,
		Tasks:          state.taskRegistry,
		taskExecutor:   state.taskExecutor,
		Scheduler:      schedulerEngine,
		Logs:           state.logStream,
		LogRepository:  logRepository,
		Console:        console.NewStream(1000, 2*1024*1024),
		launcherTokens: newLauncherTokenStore(time.Now, 5*time.Minute),
		loginFailures:  newLoginFailureTracker(time.Now),
	}, nil
}

func buildAppPlugins(
	state appBuildState,
	platform appPlatform,
	renderRunner render.Runner,
	executeLocalAction func(context.Context, string, string, runtime.Action) (map[string]any, error),
) (appPlugins, error) {
	adapterShell := adapter.New(state.core.Config.OneBot, state.core.Logger)
	runtimeOptions := runtime.Options{
		Console:                    platform.Console,
		RedactText:                 state.managementRedact,
		StderrRateLimitBytesPerSec: state.core.Config.Runtime.StderrRateLimitBytesPerSec,
		ExecuteLocalAction:         executeLocalAction,
	}
	runtimeRegistry := newRuntimeRegistry(state.core.Logger, runtimeOptions)
	replyTargets := newReplyTargetCache(defaultReplyTargetCacheSize)
	eventDispatcher := dispatch.New(state.core.Logger, adapterShell, replyTargets, state.core.Config.Runtime.MaxPendingEventsPerPlugin)
	eventBridge := bridge.New(state.core.Logger, newDispatcherRuntimeClient(eventDispatcher), adapterShell, replyTargets)

	pluginRepository, err := plugins.NewSQLiteRepository(platform.Storage)
	if err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, fmt.Errorf("create plugin repository: %w", err)
	}
	pluginKVRepository, err := pluginkv.NewSQLiteRepository(platform.Storage)
	if err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, fmt.Errorf("create plugin kv repository: %w", err)
	}
	webhookRegistry := newPluginWebhookRegistry()
	pluginConfigRepository, err := pluginconfig.NewSQLiteRepository(platform.Storage)
	if err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, fmt.Errorf("create plugin config repository: %w", err)
	}
	pluginFileService := pluginfile.NewService(filepath.Join(filepath.Dir(platform.Storage.Path), "plugins"))
	renderBrowserPath := prepareRenderBrowserPath(context.Background(), state.core.Logger, state.discoverySpec.repoRoot, state.core.Config.Render.BrowserPath)
	renderService, err := render.NewService(render.Options{
		RepoRoot:           state.discoverySpec.repoRoot,
		OutputRoot:         filepath.Join(filepath.Dir(platform.Storage.Path), "render"),
		Runner:             renderRunner,
		WorkerCount:        state.core.Config.Render.WorkerCount,
		BrowserArgs:        state.core.Config.Render.BrowserArgs,
		BrowserPath:        renderBrowserPath,
		QueueMaxLength:     state.core.Config.Render.QueueMaxLength,
		QueueWaitTimeout:   time.Duration(state.core.Config.Render.QueueWaitTimeoutSeconds) * time.Second,
		RenderTimeout:      time.Duration(state.core.Config.Render.TimeoutSeconds) * time.Second,
		MaxRenderDataBytes: int(maxManagementJSONBodyBytes),
	})
	if err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, fmt.Errorf("create render service: %w", err)
	}
	blacklistRepo := permission.NewSQLiteBlacklistRepository(platform.Storage.Read, platform.Storage.Write)

	desiredStates, err := pluginRepository.LoadDesiredStates(context.Background())
	if err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, fmt.Errorf("load persisted plugin desired_state: %w", err)
	}
	if packageLoader, ok := any(pluginRepository).(plugins.PackageMetadataLoader); ok {
		packageMetadata, err := packageLoader.LoadAllPackageMetadata(context.Background())
		if err != nil {
			_ = platform.Storage.Close()
			return appPlugins{}, fmt.Errorf("load plugin package metadata: %w", err)
		}
		state.pluginCatalog.Replace(plugins.ApplyPackageMetadata(state.pluginCatalog.List(), packageMetadata))
	}
	state.pluginCatalog.ApplyDesiredStates(desiredStates)
	cleanupOrphanedInstallDirs(state.core.Logger, state.discoverySpec.roots)

	pluginInstallService, err := plugins.NewInstallService(
		state.core.Logger,
		state.taskRegistry,
		state.pluginCatalog,
		pluginRepository,
		state.pluginValidator,
		state.discoverySpec.repoRoot,
		state.discoverySpec.roots,
		time.Duration(state.core.Config.Runtime.DependencyInstallTimeoutSecs)*time.Second,
	)
	if err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, fmt.Errorf("create plugin install service: %w", err)
	}
	pluginUninstallService, err := plugins.NewUninstallService(
		state.core.Logger,
		state.taskRegistry,
		state.pluginCatalog,
		pluginRepository,
		state.pluginValidator,
		state.discoverySpec.repoRoot,
		state.discoverySpec.roots,
		nil,
	)
	if err != nil {
		_ = platform.Storage.Close()
		return appPlugins{}, fmt.Errorf("create plugin uninstall service: %w", err)
	}

	return appPlugins{
		Plugins:           state.pluginCatalog,
		Adapter:           adapterShell,
		Bridge:            eventBridge,
		Dispatcher:        eventDispatcher,
		Runtimes:          runtimeRegistry,
		replyTargets:      replyTargets,
		outboundSender:    adapterShell,
		PluginInstaller:   pluginInstallService,
		PluginUninstaller: pluginUninstallService,
		pluginRepository:  pluginRepository,
		pluginConfig:      pluginConfigRepository,
		pluginFiles:       pluginFileService,
		pluginKV:          pluginKVRepository,
		grantRepository:   pluginRepository,
		blacklistRepo:     blacklistRepo,
		permissionChecker: newPermissionChecker(state.core.Config, blacklistRepo),
		webhooks:          webhookRegistry,
		renderer:          renderService,
		commandParser:     newCommandParser(state.core.Config),
		pluginLogLimiter:  newPluginLogLimiter(state.core.Config),
	}, nil
}

func buildAppHTTPServer(application *App) (http.Handler, *http.Server) {
	router := chi.NewRouter()
	router.Use(httpapi.WithRequestContext(application.Logger))

	router.Get("/healthz", health.NewLivenessHandler())
	router.Get("/readyz", health.NewReadinessHandler(func() health.ReadinessReport {
		return application.currentReadiness()
	}))
	router.Post("/api/setup/admin", application.handleSetupAdmin())
	router.Get("/api/setup/status", application.handleSetupStatus())
	router.Post("/api/session/login", application.handleSessionLogin())
	router.Post("/api/session/launcher-token", application.handleLauncherTokenIssue())
	router.Post("/api/session/launcher-admission", application.handleLauncherAdmission())
	router.Post("/api/webhooks/{plugin_id}/{route}", application.handlePluginWebhook())

	router.Group(func(r chi.Router) {
		r.Use(RequireAuth(application.Auth))
		r.Delete("/api/session", application.handleSessionLogout())
		r.Get("/api/config", application.handleConfigGet())
		r.Put("/api/config", application.handleConfigPut())
		r.Get("/api/logs", application.handleLogsList())
		r.Get("/api/system/status", application.handleSystemStatus())
		r.Post("/api/system/shutdown", application.handleSystemShutdown())
		r.Post("/api/system/backup", application.handleSystemBackup())
		r.Post("/api/system/recovery/recheck", application.handleSystemRecoveryRecheck())
		r.Post("/api/system/recovery/confirm", application.handleSystemRecoveryConfirm())
		r.Post("/api/system/runtime/bootstrap", application.handleSystemRuntimeBootstrap())
		r.Get("/api/system/diagnostics/export", application.handleSystemDiagnosticsExport())
		r.Post("/api/system/render/preview", application.handleSystemRenderPreview())
		r.Get("/api/system/render/artifacts/{artifact_id}", application.handleSystemRenderArtifact())
		r.Get("/api/tasks", application.handleTaskList())
		r.Get("/api/tasks/{task_id}", application.handleTaskDetail())
		r.Post("/api/tasks/{task_id}/cancel", application.handleTaskCancel())
		r.Get("/ws/events", application.handleEventsWebSocket())
		r.Get("/ws/tasks", application.handleTasksWebSocket())
		r.Get("/ws/logs", application.handleLogsWebSocket())
		r.Get("/ws/plugins/{id}/console", application.handlePluginConsoleWebSocket())
		plugins.RegisterRoutes(r, application.Plugins, application.Tasks, application.pluginRepository, application.PluginInstaller, application.pluginLifecycle, application.PluginUninstaller, application.grantRepository)
	})
	router.NotFound(newManagementUIHandler(application.repoRoot))

	listenAddr := net.JoinHostPort(application.Config.Server.Host, strconv.Itoa(application.Config.Server.Port))
	server := &http.Server{
		Addr:    listenAddr,
		Handler: router,
	}

	application.Logger.Info(
		"configuration loaded",
		"component", "config",
		"config_path", application.Summary.ConfigPath,
		"schema_path", application.Summary.SchemaPath,
		"server_host", application.Summary.ServerHost,
		"server_port", application.Summary.ServerPort,
		"database_engine", application.Summary.DatabaseEngine,
		"database_path", application.Summary.DatabasePath,
		"web_exposure_mode", application.Summary.WebExposureMode,
		"logging_level", application.Summary.LoggingLevel,
		"super_admin_count", application.Summary.SuperAdminCount,
		"onebot_configured", application.Summary.OneBotConfigured,
		"onebot_endpoint", application.Summary.OneBotEndpoint,
	)
	application.Logger.Info(
		"http server configured",
		"component", "app",
		"listen_addr", listenAddr,
	)
	for _, issue := range application.renderer.Diagnostics() {
		application.Logger.Warn(
			"render resource issue detected",
			"component", "render",
			"code", issue.Code,
			"severity", issue.Severity,
			"summary", issue.Summary,
			"remediation", issue.Remediation,
		)
	}

	return router, server
}

func prepareRenderBrowserPath(ctx context.Context, logger *slog.Logger, repoRoot string, configuredPath string) string {
	browserPath := strings.TrimSpace(configuredPath)
	if browserPath != "" {
		return browserPath
	}

	managedBrowserPath, err := resolveManagedRenderBrowserPath(ctx, repoRoot)
	if err != nil {
		if logger != nil {
			logger.Warn(
				"managed chromium bootstrap pending",
				"component", "render",
				"code", "platform.resource_missing",
				"err", err.Error(),
			)
		}
		return ""
	}

	if logger != nil {
		logger.Info(
			"managed chromium bootstrap ready",
			"component", "render",
			"browser_path", managedBrowserPath,
		)
	}
	return managedBrowserPath
}
