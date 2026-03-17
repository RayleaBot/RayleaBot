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

	"rayleabot/server/internal/config"
	"rayleabot/server/internal/health"
	"rayleabot/server/internal/logging"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/schema"
	"rayleabot/server/internal/tasks"
)

type Options struct {
	ConfigPath string
	SchemaPath string
}

type App struct {
	Config    config.Config
	Summary   config.Summary
	Logger    *slog.Logger
	Tasks     *tasks.Registry
	Plugins   *plugins.Catalog
	readiness health.ReadinessReport
	router    http.Handler
	server    *http.Server
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
	pluginCatalog, err := discoverPlugins(options.SchemaPath, logger)
	if err != nil {
		return nil, err
	}

	readiness := health.ReadinessReport{
		Status: "ready",
		Checks: map[string]string{
			"config": "ok",
		},
	}

	router := chi.NewRouter()
	router.Get("/healthz", health.NewLivenessHandler())
	router.Get("/readyz", health.NewReadinessHandler(func() health.ReadinessReport {
		return readiness
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

	return &App{
		Config:    cfg,
		Summary:   summary,
		Logger:    logger,
		Tasks:     taskRegistry,
		Plugins:   pluginCatalog,
		readiness: readiness,
		router:    router,
		server:    server,
	}, nil
}

func (a *App) Handler() http.Handler {
	return a.router
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return a.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("listen on %s: %w", a.server.Addr, err)
		}
		return nil
	}
}

func discoverPlugins(configSchemaPath string, logger *slog.Logger) (*plugins.Catalog, error) {
	repoRoot, pluginSchemaPath, roots, err := pluginDiscoveryContext(configSchemaPath)
	if err != nil {
		return nil, err
	}

	validator, err := schema.Compile(pluginSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("compile plugin manifest schema %s: %w", pluginSchemaPath, err)
	}

	snapshots, _, err := plugins.Discover(plugins.DiscoverOptions{
		Validator: validator,
		Roots:     roots,
		RepoRoot:  repoRoot,
		Logger:    logger,
	})
	if err != nil {
		return nil, fmt.Errorf("discover plugins: %w", err)
	}

	return plugins.NewCatalog(snapshots), nil
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
