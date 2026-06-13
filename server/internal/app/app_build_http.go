package app

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
)

func buildAppHTTPServer(deps httpServerDeps) (http.Handler, *http.Server) {
	router := chi.NewRouter()
	router.Use(httpapi.WithRequestContext(deps.state.Logger))

	registerAppPublicRoutes(router, deps)
	router.Group(func(r chi.Router) {
		r.Use(RequireAuth(deps.auth))
		registerAppProtectedRoutes(r, deps)
	})
	router.NotFound(newManagementUIHandler(deps.state.repoRoot))

	listenAddr := net.JoinHostPort(deps.state.Config.Server.Host, strconv.Itoa(deps.state.Config.Server.Port))
	server := &http.Server{
		Addr:              listenAddr,
		Handler:           router,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	logConfiguredServer(deps.state, deps.renderer, listenAddr)
	return router, server
}

func logConfiguredServer(state *appRuntimeState, renderer *render.Service, listenAddr string) {
	state.Logger.Info(
		"configuration loaded",
		"component", "config",
		"config_path", state.Summary.ConfigPath,
		"schema_path", state.Summary.SchemaPath,
		"server_host", state.Summary.ServerHost,
		"server_port", state.Summary.ServerPort,
		"database_engine", state.Summary.DatabaseEngine,
		"database_path", state.Summary.DatabasePath,
		"web_exposure_mode", state.Summary.WebExposureMode,
		"logging_level", state.Summary.LoggingLevel,
		"super_admin_count", state.Summary.SuperAdminCount,
		"onebot_configured", state.Summary.OneBotConfigured,
		"onebot_endpoint", state.Summary.OneBotEndpoint,
	)
	state.Logger.Info(
		"http server configured",
		"component", "app",
		"listen_addr", listenAddr,
	)
	for _, issue := range renderer.Diagnostics() {
		state.Logger.Warn(
			"render resource issue detected",
			"component", "render",
			"code", issue.Code,
			"severity", issue.Severity,
			"summary", issue.Summary,
			"remediation", issue.Remediation,
		)
	}
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
