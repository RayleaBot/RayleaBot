package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/auth"
	"rayleabot/server/internal/bridge"
	"rayleabot/server/internal/config"
	"rayleabot/server/internal/console"
	"rayleabot/server/internal/health"
	"rayleabot/server/internal/logging"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/runtime"
	"rayleabot/server/internal/schema"
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
	Config          config.Config
	Summary         config.Summary
	Logger          *slog.Logger
	Tasks           *tasks.Registry
	Plugins         *plugins.Catalog
	Auth            *auth.Manager
	Storage         *storage.Store
	Logs            *logging.Stream
	Console         *console.Stream
	Adapter         *adapter.Shell
	Bridge          *bridge.Bridge
	Runtime         *runtime.Manager
	PluginInstaller plugins.InstallCoordinator
	repoRoot        string
	router          http.Handler
	server          *http.Server
	startedAt       time.Time
	launcherTokens  *launcherTokenStore
	shuttingDown    atomic.Bool
	runCancelMu     sync.Mutex
	runCancel       context.CancelFunc
	shutdownOnce    sync.Once
}

func New(options Options) (*App, error) {
	cfg, summary, err := config.Load(options.ConfigPath, options.SchemaPath)
	if err != nil {
		return nil, err
	}

	managementRedactor := buildManagementRedactor(cfg)
	logger, logStream, err := logging.NewWithStream(cfg.Logging.Level, managementRedactor.Redact)
	if err != nil {
		return nil, err
	}

	taskRegistry := tasks.NewRegistry()
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
	runtimeManager := runtime.New(logger, runtime.Options{
		Console:                    consoleStream,
		RedactText:                 managementRedactor.Redact,
		StderrRateLimitBytesPerSec: cfg.Runtime.StderrRateLimitBytesPerSec,
	})
	eventBridge := bridge.New(logger, runtimeManager, adapterShell)
	databasePath, err := resolveDatabasePath(options.ConfigPath, cfg.Database.Path)
	if err != nil {
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
	authOptions := append([]auth.Option{
		auth.WithRepository(authRepository),
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
	pluginRepository, err := plugins.NewSQLiteRepository(storageStore)
	if err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("create plugin repository: %w", err)
	}
	desiredStates, err := pluginRepository.LoadDesiredStates(context.Background())
	if err != nil {
		_ = storageStore.Close()
		return nil, fmt.Errorf("load persisted plugin desired_state: %w", err)
	}
	pluginCatalog.ApplyDesiredStates(desiredStates)
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

	application := &App{
		Config:          cfg,
		Summary:         summary,
		Logger:          logger,
		Tasks:           taskRegistry,
		Plugins:         pluginCatalog,
		Auth:            authManager,
		Storage:         storageStore,
		Logs:            logStream,
		Console:         consoleStream,
		Adapter:         adapterShell,
		Bridge:          eventBridge,
		Runtime:         runtimeManager,
		PluginInstaller: pluginInstallService,
		repoRoot:        discoverySpec.repoRoot,
		startedAt:       time.Now().UTC(),
		launcherTokens:  newLauncherTokenStore(time.Now, 5*time.Minute),
	}
	adapterShell.SetEventHandler(application.handleAdapterEvent)

	router := chi.NewRouter()

	// Public routes — no authentication required.
	router.Get("/healthz", health.NewLivenessHandler())
	router.Get("/readyz", health.NewReadinessHandler(func() health.ReadinessReport {
		return application.currentReadiness()
	}))
	router.Post("/api/setup/admin", application.handleSetupAdmin())
	router.Get("/api/setup/status", application.handleSetupStatus())
	router.Post("/api/session/login", application.handleSessionLogin())

	// Protected routes — require a valid session token.
	router.Group(func(r chi.Router) {
		r.Use(RequireAuth(application.Auth))
		r.Delete("/api/session", application.handleSessionLogout())
		r.Post("/api/session/launcher-token", application.handleLauncherTokenIssue())
		r.Get("/api/system/status", application.handleSystemStatus())
		r.Post("/api/system/shutdown", application.handleSystemShutdown())
		r.Get("/api/tasks", application.handleTaskList())
		r.Get("/api/tasks/{task_id}", application.handleTaskDetail())
		r.Post("/api/tasks/{task_id}/cancel", application.handleTaskCancel())
		r.Get("/ws/events", application.handleEventsWebSocket())
		r.Get("/ws/tasks", application.handleTasksWebSocket())
		r.Get("/ws/logs", application.handleLogsWebSocket())
		r.Get("/ws/plugins/{id}/console", application.handlePluginConsoleWebSocket())
		plugins.RegisterRoutes(r, pluginCatalog, taskRegistry, pluginRepository, pluginInstallService)
	})

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
	if a != nil && a.PluginInstaller != nil {
		if err := a.PluginInstaller.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close plugin install service: %w", err))
		}
		a.PluginInstaller = nil
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
		runtimeStopCtx, runtimeStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer runtimeStopCancel()
		if err := a.Runtime.Stop(runtimeStopCtx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("stop runtime manager: %w", err)
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
		runtimeStopCtx, runtimeStopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer runtimeStopCancel()
		if stopErr := a.Runtime.Stop(runtimeStopCtx); stopErr != nil && !errors.Is(stopErr, context.Canceled) {
			return fmt.Errorf("stop runtime manager after http server error: %w", stopErr)
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

	baseDir := filepath.Dir(configPath)
	resolved, err := filepath.Abs(filepath.Join(baseDir, databasePath))
	if err != nil {
		return "", fmt.Errorf("resolve database path %s: %w", databasePath, err)
	}

	return resolved, nil
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
	if a == nil || a.Adapter == nil {
		return ReadinessReportFromAdapter(adapter.Snapshot{State: adapter.StateIdle})
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
