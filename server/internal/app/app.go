package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/auth"
	"rayleabot/server/internal/bridge"
	"rayleabot/server/internal/command"
	"rayleabot/server/internal/config"
	"rayleabot/server/internal/console"
	"rayleabot/server/internal/dispatch"
	"rayleabot/server/internal/health"
	"rayleabot/server/internal/httpapi"
	"rayleabot/server/internal/logging"
	"rayleabot/server/internal/permission"
	"rayleabot/server/internal/pluginconfig"
	"rayleabot/server/internal/pluginfile"
	"rayleabot/server/internal/pluginkv"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/runtime"
	"rayleabot/server/internal/scheduler"
	"rayleabot/server/internal/schema"
	"rayleabot/server/internal/secrets"
	"rayleabot/server/internal/storage"
	"rayleabot/server/internal/tasks"
)

type Options struct {
	ConfigPath       string
	SchemaPath       string
	AuthOptions      []auth.Option
	PluginRepoRoot   string
	PluginSchemaPath string
	PluginRoots      []plugins.ScanRoot
}

type App struct {
	Config            config.Config
	Summary           config.Summary
	Logger            *slog.Logger
	LogLevel          *logging.LevelController
	Tasks             *tasks.Registry
	taskExecutor      *tasks.Executor
	Plugins           *plugins.Catalog
	Auth              *auth.Manager
	Storage           *storage.Store
	Secrets           secrets.Store
	Scheduler         *scheduler.Engine
	Logs              *logging.Stream
	LogRepository     logging.Repository
	Console           *console.Stream
	Adapter           *adapter.Shell
	Bridge            *bridge.Bridge
	Dispatcher        *dispatch.Dispatcher
	Runtimes          *runtimeRegistry
	replyTargets      *replyTargetCache
	outboundSender    outboundActionSender
	PluginInstaller   plugins.InstallCoordinator
	PluginUninstaller plugins.UninstallCoordinator
	pluginRepository  plugins.DesiredStateRepository
	pluginConfig      pluginconfig.Repository
	pluginFiles       *pluginfile.Service
	pluginKV          pluginkv.Repository
	grantRepository   plugins.GrantRepository
	blacklistRepo     permission.BlacklistRepository
	permissionChecker *permission.Checker
	pluginLifecycle   *pluginLifecycleController
	webhooks          *pluginWebhookRegistry
	renderer          *renderService
	commandParser     *command.Parser
	pluginLogLimiter  *pluginLogLimiter
	redactText        func(string) string
	repoRoot          string
	router            http.Handler
	server            *http.Server
	startedAt         time.Time
	launcherTokens    *launcherTokenStore
	loginFailures     *loginFailureTracker
	shuttingDown      atomic.Bool
	runCancelMu       sync.Mutex
	runCancel         context.CancelFunc
	shutdownOnce      sync.Once
}

func New(options Options) (*App, error) {
	cfg, summary, err := config.Load(options.ConfigPath, options.SchemaPath)
	if err != nil {
		return nil, err
	}

	managementRedactor := buildManagementRedactor(cfg)
	logger, logStream, logLevel, err := logging.NewWithStreamAndController(cfg.Logging.Level, managementRedactor.Redact)
	if err != nil {
		return nil, err
	}

	taskRegistry := tasks.NewRegistry()
	taskExecutor := tasks.NewExecutor(taskRegistry, 5*time.Minute)
	discoverySpec, err := resolvePluginDiscovery(options)
	if err != nil {
		return nil, err
	}
	pluginValidator, err := schema.Compile(discoverySpec.pluginSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("compile plugin manifest schema %s: %w", discoverySpec.pluginSchemaPath, err)
	}
	snapshots, _, err := plugins.Discover(plugins.DiscoverOptions{
		Validator: pluginValidator,
		Roots:     discoverySpec.roots,
		RepoRoot:  discoverySpec.repoRoot,
		Logger:    logger,
	})
	if err != nil {
		return nil, err
	}
	pluginCatalog := plugins.NewCatalog(snapshots)
	adapterShell := adapter.New(cfg.OneBot, logger)
	consoleStream := console.NewStream(1000, 2*1024*1024)
	var application *App
	runtimeOptions := runtime.Options{
		Console:                    consoleStream,
		RedactText:                 managementRedactor.Redact,
		StderrRateLimitBytesPerSec: cfg.Runtime.StderrRateLimitBytesPerSec,
		ExecuteLocalAction: func(ctx context.Context, pluginID, requestID string, action runtime.Action) (map[string]any, error) {
			if application == nil {
				return nil, &runtime.Error{
					Code:    "plugin.internal_error",
					Message: "plugin local action executor is not available",
				}
			}
			return application.executeLocalAction(ctx, pluginID, requestID, action)
		},
	}
	runtimeRegistry := newRuntimeRegistry(logger, runtimeOptions)
	replyTargets := newReplyTargetCache(defaultReplyTargetCacheSize)
	eventDispatcher := dispatch.New(logger, adapterShell, replyTargets, cfg.Runtime.MaxPendingEventsPerPlugin)
	eventBridge := bridge.New(logger, newDispatcherRuntimeClient(eventDispatcher), adapterShell, replyTargets)
	databasePath, err := resolveDatabasePath(options.ConfigPath, cfg.Database.Path)
	if err != nil {
		return nil, err
	}
	if err := migrateLegacyDataRoot(logger, options.ConfigPath, cfg.Database.Path); err != nil {
		return nil, err
	}
	storageStore, err := storage.Open(databasePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite store: %w", err)
	}
	authRepository, err := auth.NewSQLiteRepository(storageStore)
	if err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("create auth repository: %w", err)
	}
	secretStore, err := secrets.NewSQLiteStore(storageStore)
	if err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("create secret store: %w", err)
	}
	sessionSigningKey, signingKeyCreated, err := ensureSessionSigningKey(context.Background(), secretStore)
	if err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("prepare session signing key: %w", err)
	}
	if signingKeyCreated {
		persistedSessions, err := authRepository.LoadSessions(context.Background())
		if err != nil {
			_ = storageStore.Close()
			return nil, fmt.Errorf("load persisted sessions for signing key rotation: %w", err)
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
					return nil, fmt.Errorf("invalidate persisted sessions after signing key rotation: %w", err)
				}
			}
		}
	}
	authOptions := append([]auth.Option{
		auth.WithRepository(authRepository),
		auth.WithSigningKey(sessionSigningKey),
	}, options.AuthOptions...)
	authManager, err := auth.NewManager(auth.Config{
		SessionTTLDays: cfg.Auth.SessionTTLDays,
		SlidingRenewal: cfg.Auth.SlidingRenewal,
		MaxSessions:    cfg.Auth.MaxSessions,
	}, authOptions...)
	if err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("create auth manager: %w", err)
	}
	taskRepository, err := tasks.NewSQLiteRepository(storageStore)
	if err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("create task repository: %w", err)
	}
	taskRegistry.SetRepository(taskRepository)
	if err := taskRegistry.Hydrate(context.Background()); err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("hydrate task registry: %w", err)
	}
	logRepository, err := logging.NewSQLiteRepository(storageStore)
	if err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("create logging repository: %w", err)
	}
	logStream.SetRepository(logRepository, cfg.Logging.RetentionDays)
	if cfg.Logging.RetentionDays > 0 {
		if err := logRepository.PruneOlderThan(context.Background(), time.Now().AddDate(0, 0, -cfg.Logging.RetentionDays)); err != nil {
			_ = storageStore.Close()
			return nil, fmt.Errorf("prune persisted management logs: %w", err)
		}
	}
	pluginRepository, err := plugins.NewSQLiteRepository(storageStore)
	if err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("create plugin repository: %w", err)
	}
	pluginKVRepository, err := pluginkv.NewSQLiteRepository(storageStore)
	if err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("create plugin kv repository: %w", err)
	}
	webhookRegistry := newPluginWebhookRegistry()
	pluginConfigRepository, err := pluginconfig.NewSQLiteRepository(storageStore)
	if err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("create plugin config repository: %w", err)
	}
	pluginFileService := pluginfile.NewService(filepath.Join(filepath.Dir(databasePath), "plugins"))
	renderService := newRenderService(filepath.Join(filepath.Dir(databasePath), "render"))
	blacklistRepo := permission.NewSQLiteBlacklistRepository(storageStore.Read, storageStore.Write)
	schedulerRepo, err := scheduler.NewSQLiteRepository(storageStore)
	if err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("create scheduler repository: %w", err)
	}
	schedulerEngine, err := scheduler.New(scheduler.Options{
		Repository: schedulerRepo,
		Logger:     logger,
		Trigger: func(ctx context.Context, job scheduler.Job) {
			if application != nil && application.pluginLifecycle != nil {
				application.pluginLifecycle.HandleSchedulerTrigger(ctx, job)
			}
		},
		Timezone: cfg.Runtime.SchedulerTimezone,
	})
	if err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("create scheduler engine: %w", err)
	}
	if err := schedulerEngine.Hydrate(context.Background()); err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("hydrate scheduler: %w", err)
	}
	desiredStates, err := pluginRepository.LoadDesiredStates(context.Background())
	if err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("load persisted plugin desired_state: %w", err)
	}
	if packageLoader, ok := any(pluginRepository).(plugins.PackageMetadataLoader); ok {
		packageMetadata, err := packageLoader.LoadAllPackageMetadata(context.Background())
		if err != nil {
			_ = storageStore.Close()
			return nil, fmt.Errorf("load plugin package metadata: %w", err)
		}
		pluginCatalog.Replace(plugins.ApplyPackageMetadata(pluginCatalog.List(), packageMetadata))
	}
	pluginCatalog.ApplyDesiredStates(desiredStates)
	cleanupOrphanedInstallDirs(logger, discoverySpec.roots)
	pluginInstallService, err := plugins.NewInstallService(
		logger,
		taskRegistry,
		pluginCatalog,
		pluginRepository,
		pluginValidator,
		discoverySpec.repoRoot,
		discoverySpec.roots,
		time.Duration(cfg.Runtime.DependencyInstallTimeoutSecs)*time.Second,
	)
	if err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("create plugin install service: %w", err)
	}

	pluginUninstallService, err := plugins.NewUninstallService(
		logger,
		taskRegistry,
		pluginCatalog,
		pluginRepository,
		pluginValidator,
		discoverySpec.repoRoot,
		discoverySpec.roots,
		nil, // stopPlugin callback wired after application is created
	)
	if err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("create plugin uninstall service: %w", err)
	}

	application = &App{
		Config:            cfg,
		Summary:           summary,
		Logger:            logger,
		LogLevel:          logLevel,
		Tasks:             taskRegistry,
		taskExecutor:      taskExecutor,
		Plugins:           pluginCatalog,
		Auth:              authManager,
		Storage:           storageStore,
		Secrets:           secretStore,
		Scheduler:         schedulerEngine,
		Logs:              logStream,
		LogRepository:     logRepository,
		Console:           consoleStream,
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
		permissionChecker: newPermissionChecker(cfg, blacklistRepo),
		webhooks:          webhookRegistry,
		renderer:          renderService,
		commandParser:     newCommandParser(cfg),
		pluginLogLimiter:  newPluginLogLimiter(cfg),
		redactText:        managementRedactor.Redact,
		repoRoot:          discoverySpec.repoRoot,
		startedAt:         time.Now().UTC(),
		launcherTokens:    newLauncherTokenStore(time.Now, 5*time.Minute),
		loginFailures:     newLoginFailureTracker(time.Now),
	}
	application.pluginLifecycle = newPluginLifecycleController(application)
	pluginUninstallService.SetStopPlugin(application.pluginLifecycle.stopAndResetPlugin)
	runtimeRegistry.SetOnCrash(application.pluginLifecycle.handleCrash)
	adapterShell.SetEventHandler(application.handleAdapterEvent)
	adapterShell.SetReadyHandler(application.handleAdapterReady)

	router := chi.NewRouter()
	router.Use(httpapi.WithRequestContext(application.Logger))

	// Public routes — no authentication required.
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

	// Protected routes — require a valid session token.
	router.Group(func(r chi.Router) {
		r.Use(RequireAuth(application.Auth))
		r.Delete("/api/session", application.handleSessionLogout())
		r.Get("/api/config", application.handleConfigGet())
		r.Put("/api/config", application.handleConfigPut())
		r.Get("/api/logs", application.handleLogsList())
		r.Get("/api/system/status", application.handleSystemStatus())
		r.Post("/api/system/shutdown", application.handleSystemShutdown())
		r.Post("/api/system/backup", application.handleSystemBackup())
		r.Get("/api/system/diagnostics/export", application.handleSystemDiagnosticsExport())
		r.Get("/api/tasks", application.handleTaskList())
		r.Get("/api/tasks/{task_id}", application.handleTaskDetail())
		r.Post("/api/tasks/{task_id}/cancel", application.handleTaskCancel())
		r.Get("/ws/events", application.handleEventsWebSocket())
		r.Get("/ws/tasks", application.handleTasksWebSocket())
		r.Get("/ws/logs", application.handleLogsWebSocket())
		r.Get("/ws/plugins/{id}/console", application.handlePluginConsoleWebSocket())
		plugins.RegisterRoutes(r, pluginCatalog, taskRegistry, pluginRepository, pluginInstallService, application.pluginLifecycle, pluginUninstallService, pluginRepository)
	})
	router.NotFound(newManagementUIHandler(application.repoRoot))

	listenAddr := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
	server := &http.Server{
		Addr:    listenAddr,
		Handler: router,
	}

	logger.Info(
		"configuration loaded",
		"component", "config",
		"config_path", summary.ConfigPath,
		"schema_path", summary.SchemaPath,
		"server_host", summary.ServerHost,
		"server_port", summary.ServerPort,
		"database_engine", summary.DatabaseEngine,
		"database_path", summary.DatabasePath,
		"web_exposure_mode", summary.WebExposureMode,
		"logging_level", summary.LoggingLevel,
		"super_admin_count", summary.SuperAdminCount,
		"onebot_configured", summary.OneBotConfigured,
		"onebot_endpoint", summary.OneBotEndpoint,
	)
	logger.Info(
		"http server configured",
		"component", "app",
		"listen_addr", listenAddr,
	)

	application.router = router
	application.server = server
	return application, nil
}

func (a *App) Handler() http.Handler {
	return a.router
}

func (a *App) Close() error {
	var errs []error
	if a != nil && a.Runtimes != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := a.Runtimes.StopAll(stopCtx); err != nil {
			errs = append(errs, fmt.Errorf("stop runtime managers: %w", err))
		}
		cancel()
		a.Runtimes = nil
	}
	if a != nil && a.Dispatcher != nil {
		a.Dispatcher.Close()
		a.Dispatcher = nil
	}
	if a != nil && a.PluginInstaller != nil {
		if err := a.PluginInstaller.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close plugin install service: %w", err))
		}
		a.PluginInstaller = nil
	}
	if a != nil && a.taskExecutor != nil {
		if err := a.taskExecutor.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close task executor: %w", err))
		}
		a.taskExecutor = nil
	}
	if a != nil && a.PluginUninstaller != nil {
		if closer, ok := a.PluginUninstaller.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close plugin uninstall service: %w", err))
			}
		}
		a.PluginUninstaller = nil
	}
	if err := a.closeStorage(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	runCtx, cancel := context.WithCancel(ctx)
	a.setRunCancel(cancel)
	defer a.clearRunCancel()

	a.Adapter.Start(runCtx)
	a.Scheduler.Start(runCtx)

	go func() {
		a.Logger.Info("http server starting", "component", "app", "listen_addr", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-runCtx.Done():
		a.shuttingDown.Store(true)
		a.Logger.Info("http server shutting down", "component", "app", "listen_addr", a.server.Addr)
		a.Scheduler.Stop()
		runtimeStopCtx, runtimeStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer runtimeStopCancel()
		if err := a.Runtimes.StopAll(runtimeStopCtx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("stop runtime managers: %w", err)
		}

		adapterStopCtx, adapterStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer adapterStopCancel()
		if err := a.Adapter.Stop(adapterStopCtx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("stop adapter shell: %w", err)
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return a.Close()
	case err := <-errCh:
		a.Scheduler.Stop()
		runtimeStopCtx, runtimeStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer runtimeStopCancel()
		if stopErr := a.Runtimes.StopAll(runtimeStopCtx); stopErr != nil && !errors.Is(stopErr, context.Canceled) {
			return fmt.Errorf("stop runtime managers after http server error: %w", stopErr)
		}

		adapterStopCtx, adapterStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer adapterStopCancel()
		if stopErr := a.Adapter.Stop(adapterStopCtx); stopErr != nil && !errors.Is(stopErr, context.Canceled) {
			return fmt.Errorf("stop adapter shell after http server error: %w", stopErr)
		}

		closeErr := a.Close()

		if err != nil {
			if closeErr != nil {
				return errors.Join(fmt.Errorf("listen on %s: %w", a.server.Addr, err), closeErr)
			}
			return fmt.Errorf("listen on %s: %w", a.server.Addr, err)
		}
		return closeErr
	}
}

func (a *App) setRunCancel(cancel context.CancelFunc) {
	a.runCancelMu.Lock()
	defer a.runCancelMu.Unlock()
	a.runCancel = cancel
}

func (a *App) clearRunCancel() {
	a.runCancelMu.Lock()
	defer a.runCancelMu.Unlock()
	a.runCancel = nil
}

func (a *App) requestShutdown() {
	if a == nil {
		return
	}

	a.shuttingDown.Store(true)
	a.shutdownOnce.Do(func() {
		a.runCancelMu.Lock()
		cancel := a.runCancel
		a.runCancelMu.Unlock()
		if cancel != nil {
			cancel()
		}
	})
}

func resolveDatabasePath(configPath, databasePath string) (string, error) {
	if filepath.IsAbs(databasePath) {
		return filepath.Clean(databasePath), nil
	}

	repoRoot, err := resolveRuntimeRoot(configPath)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.Abs(filepath.Join(repoRoot, databasePath))
	if err != nil {
		return "", fmt.Errorf("resolve database path %s: %w", databasePath, err)
	}

	return resolved, nil
}

func resolveRuntimeRoot(configPath string) (string, error) {
	absoluteConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return "", fmt.Errorf("resolve runtime root from %s: %w", configPath, err)
	}

	return filepath.Dir(filepath.Dir(absoluteConfigPath)), nil
}

func resolveLegacyDatabasePath(configPath, databasePath string) (string, error) {
	if filepath.IsAbs(databasePath) {
		return filepath.Clean(databasePath), nil
	}

	configDir := filepath.Dir(configPath)
	resolved, err := filepath.Abs(filepath.Join(configDir, databasePath))
	if err != nil {
		return "", fmt.Errorf("resolve legacy database path %s: %w", databasePath, err)
	}

	return resolved, nil
}

func migrateLegacyDataRoot(logger *slog.Logger, configPath, databasePath string) error {
	if filepath.IsAbs(databasePath) {
		return nil
	}

	canonicalDatabasePath, err := resolveDatabasePath(configPath, databasePath)
	if err != nil {
		return err
	}
	legacyDatabasePath, err := resolveLegacyDatabasePath(configPath, databasePath)
	if err != nil {
		return err
	}
	if canonicalDatabasePath == legacyDatabasePath {
		return nil
	}

	canonicalDataRoot := filepath.Dir(canonicalDatabasePath)
	legacyDataRoot := filepath.Dir(legacyDatabasePath)
	if canonicalDataRoot == legacyDataRoot {
		return nil
	}

	managedEntries := []string{
		filepath.Base(canonicalDatabasePath),
		filepath.Base(canonicalDatabasePath) + "-wal",
		filepath.Base(canonicalDatabasePath) + "-shm",
		"plugins",
		"render",
	}

	if err := os.MkdirAll(canonicalDataRoot, 0o755); err != nil {
		return fmt.Errorf("create canonical data directory %s: %w", canonicalDataRoot, err)
	}

	for _, entryName := range managedEntries {
		legacyEntryPath := filepath.Join(legacyDataRoot, entryName)
		info, statErr := os.Stat(legacyEntryPath)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return fmt.Errorf("inspect legacy data entry %s: %w", legacyEntryPath, statErr)
		}

		canonicalEntryPath := filepath.Join(canonicalDataRoot, entryName)
		if _, err := os.Stat(canonicalEntryPath); err == nil {
			if logger != nil {
				logger.Warn(
					"legacy data entry left in place because canonical target already exists",
					"component", "app",
					"legacy_path", legacyEntryPath,
					"canonical_path", canonicalEntryPath,
				)
			}
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect canonical data entry %s: %w", canonicalEntryPath, err)
		}

		if err := os.Rename(legacyEntryPath, canonicalEntryPath); err != nil {
			return fmt.Errorf("migrate legacy data entry %s to %s: %w", legacyEntryPath, canonicalEntryPath, err)
		}

		if logger != nil {
			logger.Info(
				"migrated legacy data entry to canonical data root",
				"component", "app",
				"legacy_path", legacyEntryPath,
				"canonical_path", canonicalEntryPath,
				"is_dir", info.IsDir(),
			)
		}
	}

	removeEmptyDir(legacyDataRoot)
	return nil
}

func removeEmptyDir(path string) {
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) > 0 {
		return
	}
	_ = os.Remove(path)
}

func (a *App) closeStorage() error {
	if a == nil || a.Storage == nil {
		return nil
	}

	if err := a.Storage.Close(); err != nil {
		return fmt.Errorf("close sqlite store: %w", err)
	}

	a.Storage = nil
	return nil
}

type pluginDiscoverySpec struct {
	repoRoot         string
	pluginSchemaPath string
	roots            []plugins.ScanRoot
}

func resolvePluginDiscovery(options Options) (pluginDiscoverySpec, error) {
	if len(options.PluginRoots) > 0 || options.PluginRepoRoot != "" || options.PluginSchemaPath != "" {
		if options.PluginRepoRoot == "" || options.PluginSchemaPath == "" || len(options.PluginRoots) == 0 {
			return pluginDiscoverySpec{}, fmt.Errorf("plugin discovery override requires repo root, schema path, and roots")
		}
		return pluginDiscoverySpec{
			repoRoot:         options.PluginRepoRoot,
			pluginSchemaPath: options.PluginSchemaPath,
			roots:            append([]plugins.ScanRoot(nil), options.PluginRoots...),
		}, nil
	}

	repoRoot, pluginSchemaPath, roots, err := pluginDiscoveryContext(options.SchemaPath)
	if err != nil {
		return pluginDiscoverySpec{}, err
	}
	return pluginDiscoverySpec{
		repoRoot:         repoRoot,
		pluginSchemaPath: pluginSchemaPath,
		roots:            roots,
	}, nil
}

func pluginDiscoveryContext(configSchemaPath string) (string, string, []plugins.ScanRoot, error) {
	absoluteConfigSchemaPath, err := filepath.Abs(configSchemaPath)
	if err != nil {
		return "", "", nil, fmt.Errorf("resolve config schema path %s: %w", configSchemaPath, err)
	}

	contractsDir := filepath.Dir(absoluteConfigSchemaPath)
	repoRoot := filepath.Dir(contractsDir)
	pluginSchemaPath := filepath.Join(contractsDir, "plugin-info.schema.json")

	roots := []plugins.ScanRoot{
		{
			Label: "plugins/builtin",
			Path:  filepath.Join(repoRoot, "plugins", "builtin"),
		},
		{
			Label: "examples/plugins",
			Path:  filepath.Join(repoRoot, "examples", "plugins"),
		},
		{
			Label: "plugins/installed",
			Path:  filepath.Join(repoRoot, "plugins", "installed"),
		},
	}

	return repoRoot, pluginSchemaPath, roots, nil
}

func (a *App) currentReadiness() health.ReadinessReport {
	if a == nil {
		return health.ReadinessReport{
			Status: "failed",
			Reason: "Management application is unavailable",
		}
	}
	if a.Auth == nil {
		return health.ReadinessReport{
			Status: "failed",
			Reason: "Management auth service is unavailable",
			Checks: map[string]string{
				"config": "ok",
			},
		}
	}
	if !a.Auth.IsBootstrapped() {
		return health.ReadinessReport{
			Status: "setup_required",
			Reason: "Initial admin setup is required",
			Checks: map[string]string{
				"config": "ok",
			},
		}
	}
	if a.Adapter == nil {
		return health.ReadinessReport{
			Status: "failed",
			Reason: "OneBot adapter is unavailable",
			Checks: map[string]string{
				"config": "ok",
			},
		}
	}

	return ReadinessReportFromAdapter(a.Adapter.Snapshot())
}

func ReadinessReportFromAdapter(snapshot adapter.Snapshot) health.ReadinessReport {
	report := health.ReadinessReport{
		Checks: map[string]string{
			"config":  "ok",
			"adapter": string(stateOrIdle(snapshot.State)),
		},
	}

	switch stateOrIdle(snapshot.State) {
	case adapter.StateConnected:
		report.Status = "ready"
	case adapter.StateAuthFailed:
		report.Status = "degraded"
		report.Reason = "OneBot authentication failed"
		report.ReasonCodes = []string{"adapter.auth_failed"}
	case adapter.StateConnecting:
		report.Status = "degraded"
		report.Reason = "OneBot reverse WebSocket is connecting"
	case adapter.StateReconnecting:
		report.Status = "degraded"
		report.Reason = "OneBot reverse WebSocket is reconnecting"
		if snapshot.LastErrorCode != "" {
			report.ReasonCodes = []string{snapshot.LastErrorCode}
		}
	case adapter.StateStopped:
		report.Status = "degraded"
		report.Reason = "OneBot adapter has stopped"
	default:
		report.Status = "degraded"
		report.Reason = "OneBot adapter has not started connecting yet"
	}

	return report
}

func stateOrIdle(state adapter.State) adapter.State {
	if state == "" {
		return adapter.StateIdle
	}

	return state
}

func cleanupOrphanedInstallDirs(logger *slog.Logger, roots []plugins.ScanRoot) {
	for _, root := range roots {
		if root.Label != "plugins/installed" {
			continue
		}
		entries, err := os.ReadDir(root.Path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if len(name) > len(".plugin-install-") && name[:len(".plugin-install-")] == ".plugin-install-" {
				orphanPath := filepath.Join(root.Path, name)
				if err := os.RemoveAll(orphanPath); err != nil {
					logger.Warn("failed to clean up orphaned install directory",
						"component", "app",
						"path", orphanPath,
						"err", err.Error(),
					)
				} else {
					logger.Info("cleaned up orphaned install directory",
						"component", "app",
						"path", orphanPath,
					)
				}
			}
		}
	}
}
