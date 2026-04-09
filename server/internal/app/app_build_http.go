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

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func buildAppHTTPServer(application *App) (http.Handler, *http.Server) {
	router := chi.NewRouter()
	router.Use(httpapi.WithRequestContext(application.Logger))

	registerAppPublicRoutes(router, application)
	router.Group(func(r chi.Router) {
		r.Use(RequireAuth(application.Auth))
		registerAppProtectedRoutes(r, application)
	})
	router.NotFound(newManagementUIHandler(application.repoRoot))

	listenAddr := net.JoinHostPort(application.Config.Server.Host, strconv.Itoa(application.Config.Server.Port))
	server := &http.Server{
		Addr:              listenAddr,
		Handler:           router,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	logConfiguredServer(application, listenAddr)
	return router, server
}

func registerAppPublicRoutes(router chi.Router, application *App) {
	router.Get("/healthz", health.NewLivenessHandler())
	router.Get("/readyz", health.NewReadinessHandler(func() health.ReadinessReport {
		return application.currentReadiness()
	}))
	router.Post("/api/setup/admin", application.handleSetupAdmin())
	router.Get("/api/setup/status", application.handleSetupStatus())
	router.Post("/api/session/login", application.handleSessionLogin())
	router.Post("/api/session/launcher-token", application.handleLauncherTokenIssue())
	router.Post("/api/session/launcher-admission", application.handleLauncherAdmission())
	router.Get("/api/protocols/onebot11/reverse-ws", application.handleProtocolOneBot11ReverseWS())
	router.Post("/api/protocols/onebot11/webhook", application.handleProtocolOneBot11Webhook())
	router.Post("/api/webhooks/{plugin_id}/{route}", application.handlePluginWebhook())
}

func registerAppProtectedRoutes(router chi.Router, application *App) {
	router.Delete("/api/session", application.handleSessionLogout())
	router.Get("/api/config", application.handleConfigGet())
	router.Put("/api/config", application.handleConfigPut())
	router.Get("/api/protocols/onebot11", application.handleProtocolOneBot11Snapshot())
	router.Get("/api/logs", application.handleLogsList())
	router.Get("/api/logs/{log_id}", application.handleLogDetail())
	router.Get("/api/system/status", application.handleSystemStatus())
	router.Post("/api/system/shutdown", application.handleSystemShutdown())
	router.Post("/api/system/backup", application.handleSystemBackup())
	router.Post("/api/system/recovery/recheck", application.handleSystemRecoveryRecheck())
	router.Post("/api/system/recovery/confirm", application.handleSystemRecoveryConfirm())
	router.Post("/api/system/runtime/bootstrap", application.handleSystemRuntimeBootstrap())
	router.Get("/api/system/diagnostics/export", application.handleSystemDiagnosticsExport())
	router.Post("/api/system/render/preview", application.handleSystemRenderPreview())
	router.Get("/api/system/render/artifacts/{artifact_id}", application.handleSystemRenderArtifact())
	router.Get("/api/tasks", application.handleTaskList())
	router.Get("/api/tasks/{task_id}", application.handleTaskDetail())
	router.Post("/api/tasks/{task_id}/cancel", application.handleTaskCancel())
	router.Get("/ws/events", application.handleEventsWebSocket())
	router.Get("/ws/tasks", application.handleTasksWebSocket())
	router.Get("/ws/logs", application.handleLogsWebSocket())
	router.Get("/ws/plugins/{id}/console", application.handlePluginConsoleWebSocket())
	plugins.RegisterRoutes(router, application.Plugins, application.Tasks, application.pluginRepository, application.PluginInstaller, application.pluginLifecycle, application.PluginUninstaller, application.grantRepository)
}

func logConfiguredServer(application *App, listenAddr string) {
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
