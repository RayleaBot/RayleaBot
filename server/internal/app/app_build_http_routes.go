package app

import (
	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func registerAppPublicRoutes(router chi.Router, deps httpServerDeps) {
	router.Get("/healthz", health.NewLivenessHandler())
	router.Get("/readyz", health.NewReadinessHandler(func() health.ReadinessReport {
		return deps.systemHandler.system.CurrentReadiness()
	}))
	router.Post("/api/setup/admin", deps.authHandler.handleSetupAdmin())
	router.Get("/api/setup/status", deps.managementHandler.handleSetupStatus())
	router.Post("/api/session/login", deps.authHandler.handleSessionLogin())
	router.Get("/api/launcher/status", deps.managementHandler.handleLauncherStatus())
	router.Post("/api/launcher/shutdown", deps.managementHandler.handleLauncherShutdown())
	router.Get("/api/protocols/onebot11/reverse-ws", deps.protocolHandler.handleProtocolOneBot11ReverseWS())
	router.Post("/api/protocols/onebot11/webhook", deps.protocolHandler.handleProtocolOneBot11Webhook())
	if deps.pluginWebhooks != nil {
		deps.pluginWebhooks.RegisterPublicRoutes(router)
	}
	if deps.pluginManagementUI != nil {
		deps.pluginManagementUI.RegisterPublicRoutes(router)
	}
}

func registerAppProtectedRoutes(router chi.Router, deps httpServerDeps) {
	router.Delete("/api/session", deps.managementHandler.handleSessionLogout())
	router.Get("/api/config", deps.configHandler.handleConfigGet())
	router.Put("/api/config", deps.configHandler.handleConfigPut())
	router.Get("/api/protocols/onebot11", deps.protocolHandler.handleProtocolOneBot11Snapshot())
	router.Get("/api/protocols/onebot11/targets", deps.protocolHandler.handleProtocolOneBot11Targets())
	router.Post("/api/protocols/onebot11/identities/resolve", deps.protocolHandler.handleProtocolOneBot11IdentitiesResolve())
	router.Get("/api/protocols/onebot11/compatibility", deps.protocolHandler.handleProtocolOneBot11Compatibility())
	if deps.governanceHandler != nil {
		deps.governanceHandler.RegisterProtectedRoutes(router)
	}
	router.Get("/api/logs", deps.logHandler.handleLogsList())
	router.Get("/api/logs/{log_id}", deps.logHandler.handleLogDetail())
	router.Get("/api/system/status", deps.managementHandler.handleSystemStatus())
	router.Post("/api/system/shutdown", deps.managementHandler.handleSystemShutdown())
	router.Post("/api/system/backup", deps.systemHandler.handleSystemBackup())
	router.Post("/api/system/recovery/recheck", deps.systemHandler.handleSystemRecoveryRecheck())
	router.Post("/api/system/recovery/confirm", deps.systemHandler.handleSystemRecoveryConfirm())
	router.Post("/api/system/runtime/bootstrap", deps.systemHandler.handleSystemRuntimeBootstrap())
	router.Get("/api/system/diagnostics/export", deps.systemHandler.handleSystemDiagnosticsExport())
	if deps.metrics != nil {
		router.Get("/api/system/metrics", deps.metrics.HTTPHandler().ServeHTTP)
	}
	router.Get("/api/system/render/templates", deps.renderHandler.handleSystemRenderTemplateList())
	router.Post("/api/system/render/templates/{template_id}/preview-html", deps.renderHandler.handleSystemRenderTemplatePreviewHTML())
	router.Get("/api/system/render/templates/{template_id}/asset", deps.renderHandler.handleSystemRenderTemplateAsset())
	router.Get("/api/system/render/templates/{template_id}", deps.renderHandler.handleSystemRenderTemplateDetail())
	router.Get("/api/system/scheduler/jobs", deps.systemHandler.handleSystemSchedulerJobList())
	router.Post("/api/system/scheduler/jobs/{job_id}/trigger", deps.systemHandler.handleSystemSchedulerJobTrigger())
	router.Get("/api/third-party/accounts", deps.thirdPartyHandler.handleThirdPartyAccountList())
	router.Put("/api/third-party/accounts/{platform}/{account_id}", deps.thirdPartyHandler.handleThirdPartyAccountUpsert())
	router.Delete("/api/third-party/accounts/{platform}/{account_id}", deps.thirdPartyHandler.handleThirdPartyAccountDelete())
	router.Get("/api/third-party/monitors", deps.thirdPartyHandler.handleThirdPartyMonitorList())
	router.Get("/api/third-party/media", deps.thirdPartyHandler.handleThirdPartyMedia())
	router.Post("/api/bilibili/login/qrcode", deps.bilibiliHandler.handleBilibiliQRCodeLoginCreate())
	router.Get("/api/bilibili/login/qrcode/{login_id}", deps.bilibiliHandler.handleBilibiliQRCodeLoginPoll())
	router.Get("/api/bilibili/users/resolve", deps.bilibiliHandler.handleBilibiliUserResolve())
	router.Get("/api/bilibili/source/status", deps.bilibiliHandler.handleBilibiliSourceStatus())
	router.Post("/api/bilibili/source/restart", deps.bilibiliHandler.handleBilibiliSourceRestart())
	router.Get("/api/tasks", deps.taskHandler.handleTaskList())
	router.Get("/api/tasks/{task_id}", deps.taskHandler.handleTaskDetail())
	router.Post("/api/tasks/{task_id}/cancel", deps.taskHandler.handleTaskCancel())
	if deps.pluginManagementUI != nil {
		deps.pluginManagementUI.RegisterProtectedRoutes(router)
	}
	router.Get("/ws/events", deps.eventsWS.handleEventsWebSocket())
	router.Get("/ws/tasks", deps.tasksWS.handleTasksWebSocket())
	router.Get("/ws/logs", deps.logsWS.handleLogsWebSocket())
	router.Get("/ws/plugins/{id}/console", deps.consoleWS.handlePluginConsoleWebSocket())
	plugins.RegisterRoutes(router, deps.plugins, deps.tasks, deps.pluginRepository, deps.pluginInstaller, deps.pluginLifecycle, deps.pluginUninstaller, deps.grantRepository, currentPluginAutoGrantCapabilities(deps.state))
}

func currentPluginAutoGrantCapabilities(state *appRuntimeState) func() []string {
	return func() []string {
		if state == nil {
			return nil
		}
		return append([]string(nil), state.Config.Permission.AutoGrantCapabilities...)
	}
}
