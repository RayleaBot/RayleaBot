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
	"time"

	"github.com/go-chi/chi/v5"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/auth"
	"rayleabot/server/internal/bridge"
	"rayleabot/server/internal/config"
	"rayleabot/server/internal/health"
	"rayleabot/server/internal/logging"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/runtime"
	"rayleabot/server/internal/schema"
	"rayleabot/server/internal/tasks"
)

type Options struct {
	ConfigPath string
	SchemaPath string
}

type App struct {
	Config   config.Config
	Summary  config.Summary
	Logger   *slog.Logger
	Tasks    *tasks.Registry
	Plugins  *plugins.Catalog
	Auth     *auth.Manager
	Adapter  *adapter.Shell
	Bridge   *bridge.Bridge
	Runtime  *runtime.Manager
	repoRoot string
	router   http.Handler
	server   *http.Server
}

func New(options Options) (*App, error) {
	cfg, summary, err := config.Load(options.ConfigPath, options.SchemaPath)
	if err != nil {
		return nil, err
	}

	logger, err := logging.New(cfg.Logging.Level)
	if err != nil {
		return nil, err
	}

	taskRegistry := tasks.NewRegistry()
	pluginCatalog, repoRoot, err := discoverPlugins(options.SchemaPath, logger)
	if err != nil {
		return nil, err
	}
	adapterShell := adapter.New(cfg.OneBot, logger)
	runtimeManager := runtime.New(logger)
	eventBridge := bridge.New(logger, runtimeManager)
	authManager, err := auth.NewManager(auth.Config{
		SessionTTLDays: cfg.Auth.SessionTTLDays,
		SlidingRenewal: cfg.Auth.SlidingRenewal,
		MaxSessions:    cfg.Auth.MaxSessions,
	})
	if err != nil {
		return nil, fmt.Errorf("create auth manager: %w", err)
	}

	application := &App{
		Config:   cfg,
		Summary:  summary,
		Logger:   logger,
		Tasks:    taskRegistry,
		Plugins:  pluginCatalog,
		Auth:     authManager,
		Adapter:  adapterShell,
		Bridge:   eventBridge,
		Runtime:  runtimeManager,
		repoRoot: repoRoot,
	}
	adapterShell.SetEventHandler(application.handleAdapterEvent)

	router := chi.NewRouter()
	router.Get("/healthz", health.NewLivenessHandler())
	router.Get("/readyz", health.NewReadinessHandler(func() health.ReadinessReport {
		return application.currentReadiness()
	}))
	plugins.RegisterRoutes(router, pluginCatalog)

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

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	a.Adapter.Start(ctx)

	go func() {
		a.Logger.Info("http server starting", "component", "app", "listen_addr", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
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
		return a.server.Shutdown(shutdownCtx)
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

		if err != nil {
			return fmt.Errorf("listen on %s: %w", a.server.Addr, err)
		}
		return nil
	}
}

func discoverPlugins(configSchemaPath string, logger *slog.Logger) (*plugins.Catalog, string, error) {
	repoRoot, pluginSchemaPath, roots, err := pluginDiscoveryContext(configSchemaPath)
	if err != nil {
		return nil, "", err
	}

	validator, err := schema.Compile(pluginSchemaPath)
	if err != nil {
		return nil, "", fmt.Errorf("compile plugin manifest schema %s: %w", pluginSchemaPath, err)
	}

	snapshots, _, err := plugins.Discover(plugins.DiscoverOptions{
		Validator: validator,
		Roots:     roots,
		RepoRoot:  repoRoot,
		Logger:    logger,
	})
	if err != nil {
		return nil, "", fmt.Errorf("discover plugins: %w", err)
	}

	return plugins.NewCatalog(snapshots), repoRoot, nil
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
